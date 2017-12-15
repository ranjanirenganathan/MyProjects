package models

import (
	"gopkg.in/mgo.v2/bson"
	"reflect"
	"gopkg.in/mgo.v2"
	"fmt"
)

/*
Query is used to configure your find requests on a specific collection / model in more detail.
Each query object method returns the same query reference to enable chains. After you have finished your configuration
run the exec function (see: func (*Query) Exec).

For Example:

	users := []*models.User{}

	User.Find(bson.M{"lastname":"Mustermann"}).Skip(10).Limit(5).Exec(&users)
*/
type Query struct {
	collection *mgo.Collection
	connection *Connection
	query      interface{}
	selector   interface{}
	populate   []string
	sort       []string
	limit      int
	skip       int
	multiple   bool
}

//See: http://godoc.org/labix.org/v2/mgo#Query.Select
func (q *Query) Select(selector interface{}) *Query {

	q.selector = selector

	return q
}

//See: http://godoc.org/labix.org/v2/mgo#Query.Sort
func (q *Query) Sort(fields ...string) *Query {

	q.sort = append(q.sort, fields...)

	return q
}

//See: http://godoc.org/labix.org/v2/mgo#Query.Limit
func (q *Query) Limit(limit int) *Query {

	q.limit = limit

	return q
}

//See: http://godoc.org/labix.org/v2/mgo#Query.Skip
func (q *Query) Skip(skip int) *Query {

	q.skip = skip

	return q
}

//see: http://godoc.org/gopkg.in/mgo.v2#Query.Count
func (q *Query) Count() (n int, err error) {

	return q.collection.Find(q.query).Count()
}


/*Note: Only the first relation level gets populated! This process is not recursive.
*/
func (q *Query) Populate(fields ...string) *Query {

	q.populate = append(q.populate, fields...)

	return q
}

func (q *Query) Exec(result interface{}) error {

	if result == nil {
		panic("DB: No result specified")
	}

	resultType := reflect.TypeOf(result)

	/*
	 *	Check the given result type at first to determine if its a slice or struct pointer
	 */

	//expect pointer to a slice
	if resultType.Kind() == reflect.Ptr && resultType.Elem().Kind() == reflect.Slice {

		if !q.multiple {
			panic("DB: Execution expected an IDocumentBase type!")
		}

		/*
		 *	single Query execution
		 */

		mgoQuery := q.collection.Find(q.query)

		q.extendQuery(mgoQuery)

		err := mgoQuery.All(result)

		if err == mgo.ErrNotFound {

			return &NotFoundError{&QueryError{fmt.Sprintf("No records found")}}

		} else if err != nil {

			return err

		} else {

			slice := reflect.ValueOf(result).Elem()

			for index := 0; index < slice.Len(); index++ {

				current := slice.Index(index)

				q.initWithObjectId(current)
				q.initDocument(&current, &current, q.collection, q.connection)


			}

		}
		//expect all other types - missmatch will panic later or through mgo adapter
	} else {

		if q.multiple {
			panic("DB: Execution expected a pointer to a slice!")
		}

		/*
		 *	multiple Query execution
		 */

		mgoQuery := q.collection.Find(q.query)

		q.extendQuery(mgoQuery)

		err := mgoQuery.One(result)

		if err == mgo.ErrNotFound {

			return &NotFoundError{&QueryError{fmt.Sprintf("No records found")}}

		} else if err != nil {

			return err

		}

		value := reflect.ValueOf(result)

		q.initWithObjectId(value)
		q.initDocument(&value, &value, q.collection, q.connection)

	}

	return nil
}

//extendQuery sets all native query options if specified
func (q *Query) extendQuery(mgoQuery *mgo.Query) {

	if q.selector != nil {
		mgoQuery.Select(q.selector)
	}

	if len(q.sort) > 0 {
		mgoQuery.Sort(q.sort...)
	}

	if q.limit != 0 {
		mgoQuery.Limit(q.limit)
	}

	if q.skip != 0 {
		mgoQuery.Skip(q.skip)
	}
}

//runPopulation populates all specified fields with defined struct types
func (q *Query) runPopulation(document reflect.Value) error {
	//iterate all specified population strings
	for _, populateFieldName := range q.populate {

		//check if the field name matches with a population
		if structField, ok := document.Elem().Type().FieldByName(populateFieldName); ok {

			modelTagValue := structField.Tag.Get("model")

			//check if the relation model tag is set
			if len(modelTagValue) == 0 {
				panic(fmt.Sprintf("DB: Related model tag was not set for field '%v' in type '%v'", populateFieldName, document.Elem().Type().Name()))
			}

			//build the relation type


			relatedDocument := q.connection.document(modelTagValue)
			relatedModel := q.connection.Model(relatedDocument,modelTagValue)
			field := document.Elem().FieldByName(populateFieldName)

			//check if the field is existent
			if !field.IsNil() {

				/*
				 *	This part detects the relationship type (object or slice)
				 * 	and decides what has to be set. We dont need the check for "multiple"
				 * 	here because this was already done in exec.
				 */

				switch fieldType := field.Interface().(type) {

				//one-to-one
				case bson.ObjectId:

					//find the matching document in the related collection
					relatedId := fieldType
					relationError := relatedModel.FindId(relatedId).Exec(relatedDocument)

					if relationError == mgo.ErrNotFound && relationError != nil {

						//dont set/init anything here, because nil is the correct behaviour
						return relationError

					} else {

						//populate the field
						value := reflect.ValueOf(relatedDocument)

						q.initWithObjectId(value)
						q.initDocument(&value, &value, relatedModel.Collection, relatedModel.connection)

						field.Set(value)
					}

				//one-to-many
				case []interface{}:

					//cast the object id slice to []bson.ObjectId
					idSlice := reflect.ValueOf(fieldType)
					idSliceInterface, _ := idSlice.Interface().([]interface{})

					//create a result slice with length of id slice (pointer is important for query execution!)
					resultSlice := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(relatedDocument)), idSlice.Len(), idSlice.Len())
					resultSlicePtr := reflect.New(resultSlice.Type())

					//find relation objects by searching for ids which match with entrys from id slice
					relationError := relatedModel.Find(bson.M{"_id": bson.M{"$in": &idSliceInterface}}).Exec(resultSlicePtr.Interface())

					if relationError == mgo.ErrNotFound || resultSlice.Len() == 0 {

						//in this case it is strictly necessary to init an empty slice of the document type (nil wouldnt be correct)
						field.Set(reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(relatedDocument)), 0, 0))

					} else if relationError != nil {
						return relationError

					} else {

						field.Set(resultSlicePtr.Elem())

						for index := 0; index < resultSlicePtr.Elem().Len(); index++ {

							populatedChild := resultSlicePtr.Elem().Index(index)

							q.initWithObjectId(populatedChild)
							q.initDocument(&populatedChild, &populatedChild, relatedModel.Collection, relatedModel.connection)
						}
					}

				default:

					panic("DB: unknown type stored as relation - bson.ObjectId or []bson.ObjectId expected")
				}
			}

		} else {
			panic(fmt.Sprintf("DB: Can not populate field '%v' for type '%v'. Field not found.", populateFieldName, document.Elem().Type().Name()))
		}
	}

	return nil
}


/*
 * bson object ids have to be copied again, because after passing
 * the result reference to the mgo execute function the type
 * is overwritten as interface{} again. So initWithObjectId initializes
 * bson.ObjectId and []bson.ObjectId types.
 */
func (q *Query) initWithObjectId(document reflect.Value) {

	//If there is nothing to populate, init the fields with object id types
	if len(q.populate) == 0 {

		structElement := document.Elem()
		fieldType := structElement.Type()

		//Iterate over all struct fields
		for fieldIndex := 0; fieldIndex < structElement.NumField(); fieldIndex++ {

			relationTag := fieldType.Field(fieldIndex).Tag.Get("relation")
			field := structElement.Field(fieldIndex)

			if len(relationTag) > 0 {

				if relationTag == REL_1N {

					if field.IsNil() {

						//field.Set(reflect.ValueOf(make([]bson.ObjectId, 0, 0)))

					} else {

						slice := field.Elem()
						idSlice := make([]bson.ObjectId, slice.Len(), slice.Len())

						for index := 0; index < slice.Len(); index++ {
							idSlice[index] = slice.Index(index).Elem().Interface().(bson.ObjectId)
						}

						field.Set(reflect.ValueOf(idSlice))

					}

				} else {

					if !field.IsNil() {

						objectId := field.Elem().Interface().(bson.ObjectId)
						field.Set(reflect.ValueOf(objectId))
					}

					//field.Set(reflect.Zero(reflect.TypeOf(bson.ObjectId(""))))
				}
			}
		}
	}
}

//like Model.New(), only directly for reflect types
func (q *Query) initDocument(model *reflect.Value, document *reflect.Value, collection *mgo.Collection, connection *Connection) {

	documentMethod := model.MethodByName("SetDocument")
	collectionMethod := model.MethodByName("SetCollection")
	connectionMethod := model.MethodByName("SetConnection")

	if !documentMethod.IsValid() || !collectionMethod.IsValid() || !connectionMethod.IsValid() {
		panic("Given models were not correctly initialized with 'DocumentBase' interface type")
	}

	documentInput := []reflect.Value{*document}
	collectionInput := []reflect.Value{reflect.ValueOf(collection)}
	connectionInput := []reflect.Value{reflect.ValueOf(connection)}

	documentMethod.Call(documentInput)
	collectionMethod.Call(collectionInput)
	connectionMethod.Call(connectionInput)
}