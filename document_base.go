package models

import (
	"gopkg.in/mgo.v2/bson"
	"time"
	"gopkg.in/mgo.v2"
	"fmt"
	"reflect"
	"errors"
)


type DocumentBase struct {
	document IDocumentBase   `json:"-" bson:"-"`
	collection *mgo.Collection `json:"-" bson:"-"`
	connection *Connection     `json:"-" bson:"-"`

	Id        bson.ObjectId `json:"id" bson:"_id,omitempty"`
	CreatedAt time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt" bson:"updatedAt"`

}

func (d *DocumentBase) SetCollection(collection *mgo.Collection){
	d.collection = collection
}

func (d *DocumentBase)SetDocument(document IDocumentBase){
	d.document = document
}

func (d *DocumentBase)SetConnection(connection *Connection){
	d.connection = connection
}

func (d *DocumentBase) GetId() bson.ObjectId {
	return d.Id
}

func (d *DocumentBase) SetId(id bson.ObjectId) {
	d.Id = id
}


func (d *DocumentBase) SetCreatedAt(createdAt time.Time) {
	d.CreatedAt = createdAt
}

func (d *DocumentBase) SetUpdatedAt(updatedAt time.Time) {
	d.UpdatedAt = updatedAt
}


func (d *DocumentBase) AppendError(errorList *[]error, message string) {

	*errorList = append(*errorList, errors.New(message))
}

func (d *DocumentBase) Populate(field ...string) error {

	if d.document == nil || d.collection == nil || d.connection == nil {
		panic("You have to initialize your document with *Model.New(document IDocumentBase) before using Populate()!")
	}

	query := &Query{
		collection: d.collection,
		connection: d.connection,
		query:      bson.M{},
		multiple:   false,
		populate:   field,
	}

	return query.runPopulation(reflect.ValueOf(d.document))
}


func (d *DocumentBase) Save() error {

	if d.document == nil || d.collection == nil || d.connection == nil {
		panic("You have to initialize your document with *Model.New(document IDocumentBase) before using Save()!")
	}

	// Validate document first

	/*if valid, issues := d.document.Validate(); !valid {
		return &ValidationError{&QueryError{"Document could not be validated"}, issues}
	}*/

	/*
	 * "This behavior ensures that writes performed in the old session are necessarily observed
	 * when using the new session, as long as it was a strong or monotonic session.
	 * That said, it also means that long operations may cause other goroutines using the
	 * original session to wait." see: http://godoc.org/labix.org/v2/mgo#Session.Clone
	 */

	session := d.connection.Session.Clone()
	defer session.Close()

	collections := session.DB(d.connection.Config.DatabaseName).C(d.collection.Name)
	reflectStruct := reflect.ValueOf(d.document).Elem()
	fieldType := reflectStruct.Type()
	bufferRegistry := make(map[reflect.Value]reflect.Value) //used for restoring after fields got serialized - we only save ids when not embedded

	/*
	 *	Iterate over all struct fields and determine
	 *	if there are any relations specified.
	 */
	for fieldIndex := 0; fieldIndex < reflectStruct.NumField(); fieldIndex++ {

		modelTag := fieldType.Field(fieldIndex).Tag.Get("model")       //the type which should be referenced
		relationTag := fieldType.Field(fieldIndex).Tag.Get("relation") //reference relation, e.g. one-to-one or one-to-many
		autoSaveTag := fieldType.Field(fieldIndex).Tag.Get("autosave") //flag if children of relation get automatically saved

		/*
		 *	Check if custom model and relation field tag is set,
		 *  otherwise ignore.
		 */
		if len(modelTag) > 0 {

			var fieldValue reflect.Value
			var autoSave bool
			var relation string

			field := reflectStruct.Field(fieldIndex)

			// Determine relation type for default initialization
			if relationTag == REL_11 {
				relation = REL_11
			} else if relationTag == REL_1N {
				relation = REL_1N
			} else {
				relation = REL_11 //set one-to-one as default relation
			}

			// If nil and relation one-to-many -> init field with empty slice of object ids and continue loop
			if field.IsNil() {

				if relation == REL_1N {
					field.Set(reflect.ValueOf(make([]bson.ObjectId, 0, 0)))
				}

				continue
			}

			// Determine if relation should be autosaved
			if autoSaveTag == "true" {
				autoSave = true
			} else {
				autoSave = false //set autosave default to false
			}

			// Get element of field by checking if pointer or copy
			if field.Kind() == reflect.Ptr || field.Kind() == reflect.Interface {
				fieldValue = field.Elem()
			} else {
				fieldValue = field
			}

			/*
			 *	Detect if the field is a slice, struct or string
			 *  to handle the different types of relation. Other
			 *	types are not admitted.
			 */

			// One to many
			if fieldValue.Kind() == reflect.Slice {

				if relation != REL_1N {
					panic("Relation must be '1n' when using slices!")
				}

				sliceLen := fieldValue.Len()
				idBuffer := make([]bson.ObjectId, sliceLen, sliceLen)

				// Iterate the slice
				for index := 0; index < sliceLen; index++ {

					sliceValue := fieldValue.Index(index)

					err, objectId := d.persistRelation(sliceValue, autoSave)

					if err != nil {
						return err
					}

					idBuffer[index] = objectId
				}

				/*
				 *	Store the original value and then replace
				 *  it with the generated id list. The value gets
				 *  restored after the model was saved
				 */

				bufferRegistry[field] = fieldValue
				field.Set(reflect.ValueOf(idBuffer))

				// One to one
			} else if (fieldValue.Kind() == reflect.Ptr && fieldValue.Elem().Kind() == reflect.Struct) || fieldValue.Kind() == reflect.String {

				if relation != REL_11 {
					panic("Relation must be '11' when using struct or id!")
				}

				var idBuffer bson.ObjectId

				err, objectId := d.persistRelation(fieldValue, autoSave)

				if err != nil {
					return err
				}

				idBuffer = objectId

				/*
				 *	Store the original value and then replace
				 *  it with the object id. The value gets
				 *  restored after the model was saved
				 */

				bufferRegistry[field] = fieldValue
				field.Set(reflect.ValueOf(idBuffer))

			} else {
				panic(fmt.Sprintf("DB: Following field kinds are supported for saving relations: slice, struct, string. You used %v", fieldValue.Kind()))
			}

		}

	}

	var err error
	now := time.Now()

	/*
	 *	Check if Object ID is already set.
	 * 	If yes -> Update object
	 * 	If no -> Create object
	 */
	if len(d.Id) == 0 {

		d.SetCreatedAt(now)
		d.SetUpdatedAt(now)

		d.SetId(bson.NewObjectId())

		err = collections.Insert(d.document)

		if err != nil {

			if mgo.IsDup(err) {
				err = &DuplicateError{&QueryError{fmt.Sprintf("Duplicate key")}}
			}
		}

	} else {

		d.SetUpdatedAt(now)
		_, errs := collections.UpsertId(d.Id, d.document)

		if errs != nil {

			if mgo.IsDup(errs) {
				errs = &DuplicateError{&QueryError{fmt.Sprintf("Duplicate key")}}
			} else {
				err = errs
			}
		}
	}

	/*
	 *	Restore fields which were changed
	 *	for saving progress (object deserialisation)
	 */
	for field, oldValue := range bufferRegistry {
		field.Set(oldValue)
	}

	return err
}



func (d *DocumentBase) persistRelation(value reflect.Value, autoSave bool) (error, bson.ObjectId) {

	// Detect the type of the value which is stored within the slice
	switch typedValue := value.Interface().(type) {

	// Deserialize objects to id
	case IDocumentBase:
		{
			// Save children when flag is enabled
			/*if autoSave {
				err := typedValue.Save()

				if err != nil {
					return err, bson.ObjectId("")
				}
			}*/

			objectId := typedValue.GetId()

			if !objectId.Valid() {
				panic("DB: Can not persist the relation object because the child was not saved before (invalid id).")
			}

			return nil, objectId
		}

	// Only save the id
	case bson.ObjectId:
		{
			if !typedValue.Valid() {
				panic("DB: Can not persist the relation object because the child was not saved before (invalid id).")
			}

			return nil, typedValue
		}

	case string:
		{
			if !bson.IsObjectIdHex(typedValue) {
				return &InvalidIdError{&QueryError{fmt.Sprintf("Invalid id`s given")}}, bson.ObjectId("")
			} else {
				return nil, bson.ObjectIdHex(typedValue)
			}
		}

	default:
		{
			panic(fmt.Sprintf("DB: Only type 'bson.ObjectId' and 'IDocumentBase' can be stored in slices. You used %v", value.Interface()))
		}
	}
}
