package models

import (
	"fmt"
	"errors"
	"gopkg.in/mgo.v2"
	"time"
	"gopkg.in/mgo.v2/bson"
	"reflect"
	"strings"
)

const REL_11 string = "11" // one-to-one relation
const REL_1N string = "1n" // one-to-many relation

type (
	//Simple config to create a new connection
	Config struct {
		DatabaseHosts    []string
		DatabaseName     string
		DatabaseUser     string
		DatabasePassword string
	}

	//The "Database" which stores all connections
	Connection struct {
		Config        *Config
		Session       *mgo.Session
		modelRegistry map[string]*Collection
		typeRegistry  map[string]reflect.Type

	}

	//Interface which each collection document (model) has to implement
	IDocumentBase interface {
		GetId() bson.ObjectId
		SetId(bson.ObjectId)

		SetCreatedAt(time.Time)
		SetUpdatedAt(time.Time)

		SetCollection(*mgo.Collection)
		SetDocument(document IDocumentBase)
		SetConnection(*Connection)
		Save() error

	}
)


func Connect(config *Config) (*Connection, error) {

	con := &Connection{
		Config:        config,
		Session:       nil,
		modelRegistry: make(map[string]*Collection),
		typeRegistry:  make(map[string]reflect.Type),
	}

	err := con.Open()

	return con, err
}

func (c *Connection) document(typeName string) IDocumentBase {

	typeNameLC := strings.ToLower(typeName)

	if _, ok := c.typeRegistry[typeNameLC]; ok {

		reflectType := c.typeRegistry[typeNameLC]
		document := reflect.New(reflectType).Interface().(IDocumentBase)

		c.modelRegistry[typeNameLC].New(document)

		return document
	}

	panic(fmt.Sprintf("DB: Type '%v' is not registered", typeName))
}


func (c *Connection) Model(document IDocumentBase, collectionName string) *Collection {

	if document == nil {
		panic("document can not be nil")
	}
	reflectType := reflect.TypeOf(document)
	typeName := strings.ToLower(reflectType.Elem().Name())
	collection := c.Session.DB("").C(collectionName) // empty string returns db name from dial info
	model := &Collection{collection,c}
	c.modelRegistry[typeName] = model
	c.typeRegistry[typeName] = reflectType.Elem()
	return c.modelRegistry[typeName]
}


//Opens a database connection
//This method gets called automatically from the Connect() method.
func (c *Connection) Open() (err error) {

	defer func() {
		if r := recover(); r != nil {

			if e, ok := r.(error); ok {
				err = e
			} else if e, ok := r.(string); ok {
				err = errors.New(e)
			} else {
				err = errors.New(fmt.Sprint(r))
			}
		}
	}()

	info := &mgo.DialInfo{
		Addrs:    c.Config.DatabaseHosts,
		Timeout:  3 * time.Second,
		Database: c.Config.DatabaseName,
		Username: c.Config.DatabaseUser,
		Password: c.Config.DatabasePassword,
	}

	session, err := mgo.DialWithInfo(info)

	if err != nil {
		return err
	}

	c.Session = session
	c.Session.SetMode(mgo.Monotonic, true)
	return nil
}

//Closes an existing database connection
func (c *Connection) Close() {

	if c.Session != nil {
		c.Session.Close()
	}
}