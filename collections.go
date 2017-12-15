package models

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Collection struct{
	*mgo.Collection
	connection *Connection

}



func (c *Collection) New(document IDocumentBase, content ...interface{}) (error, map[string]interface{}) {

	if document == nil {
		panic("model can not be nil")
	}

	//init collection and set pointer to its own collection (this is needed for odm operations)

	document.SetCollection(c.Collection)
	document.SetDocument(document)
	document.SetConnection(c.connection)


	return nil, nil
}



func (c *Collection) Find(query ...interface{}) *Query {

	var finalQuery interface{}

	//accept zero or one query param
	if len(query) == 0 {
		finalQuery = bson.M{}
	} else if len(query) == 1 {
		finalQuery = query[0]
	} else {
		panic("DB: Find method accepts no or maximum one query param.")
	}

	return &Query{
		query:      finalQuery,
		collection: c.Collection,
		connection: c.connection,
		multiple:   true,
	}
}


func (c *Collection) FindId(id bson.ObjectId) *Query {

	return &Query{
		collection: c.Collection,
		connection: c.connection,
		query:      bson.M{"_id": id},
		multiple:   false,
	}
}

func (c *Collection) FindOne(query ...interface{}) *Query {

	var finalQuery interface{}

	//accept zero or one query param
	if len(query) == 0 {
		finalQuery = bson.M{}
	} else if len(query) == 1 {
		finalQuery = query[0]
	} else {
		panic("DB: Find method accepts no or maximum one query param.")
	}

	return &Query{
		collection: c.Collection,
		connection: c.connection,
		query:      finalQuery,
		multiple:   false,
	}
}


