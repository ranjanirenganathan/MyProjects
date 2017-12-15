package controllers

import (
	"encoding/json"
	"example/models"
	"log"
	"time"
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

const DBName = "DBNAME"
const CollectionUsers = "COLLECTIONNAME"

type UserController struct {
	db *models.Connection
}

var (
	Database *models.Connection
)

func NewUserController(s *models.Connection) *UserController {
	return &UserController{s}
}

func responseErrorBuilder(err error, msg string, status string, c *gin.Context) {
	if err != nil {
		log.Fatal(msg, err)
		content := gin.H{
			"status": status,
			"result": msg,
		}
		c.Writer.Header().Set("Content-Type", "application/json")
		c.JSON(200, content)
	}

}

func responseErrorMessage(msg string, status string, c *gin.Context) {
	content := gin.H{
		"status": status,
		"result": msg,
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.JSON(200, content)
}

func (uc UserController) UsersList(r *gin.Context) {
	User := uc.db.Model(&models.User{}, CollectionUsers)
	users := []*models.User{}
	err := User.Find().Exec(&users)
	if err != nil {
		responseErrorBuilder(err, "Users does not exists", "404", r)
		return
	}
	r.JSON(200, users)
}

func (uc UserController) Create(c *gin.Context) {
	NewUser := uc.db.Model(&models.User{}, CollectionUsers)
	var data models.User
	params := json.NewDecoder(c.Request.Body).Decode(&data)
	if params != nil {
		c.JSON(500, gin.H{"result": "An error occured", "params": params})
		return
	}

	NewUser.New(&data)
	err := data.Save()
	if err != nil {
		responseErrorBuilder(err, "Insert failed", "403", c)

	} else {

		content := gin.H{
			"result": "Success",
			"user":   data,
		}
		c.Writer.Header().Set("Content-Type", "application/json")
		c.JSON(200, content)
	}
}

// GetUser retrieves an individual user resource
func (uc UserController) GetUser(c *gin.Context) {

	// Grab id
	id := c.Params.ByName("id")

	//Verify id is ObjectId, otherwise bail
	if !bson.IsObjectIdHex(id) {
		responseErrorMessage("ID is not a bson.ObjectId", "404", c)
		return
	}
	// Grab id
	oid := bson.ObjectIdHex(id)
	u := models.User{}
	err := uc.db.Model(&models.User{}, CollectionUsers).FindId(oid).Exec(&u)
	// Fetch user
	if err != nil {
		responseErrorMessage("Users doesn't exist", "404", c)
		return
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.JSON(200, u)
}

func (uc UserController) UpdateUser(c *gin.Context) {
	// Grab id
	id := c.Params.ByName("id")
	var data models.User

	params := json.NewDecoder(c.Request.Body).Decode(&data)
	if params != nil {
		c.JSON(500, gin.H{"result": "An error occured", "params": params})
		return
	}

	// Verify id is ObjectId, otherwise return
	if !bson.IsObjectIdHex(id) {
		responseErrorMessage("ID is not a bson.ObjectId", "404", c)
		return
	}

	// Grab id
	oid := bson.ObjectIdHex(id)
	data.SetCreatedAt(bson.ObjectId(oid).Time())
	data.SetUpdatedAt(time.Now())
	// Write the user to mongo
	err := uc.db.Model(&models.User{}, CollectionUsers).UpdateId(oid, &data)
	data.SetId(oid)

	if err != nil {
		responseErrorBuilder(err, "Insert failed", "403", c)

	} else {

		content := gin.H{
			"result": "Success",
			"user":   data,
		}
		c.Writer.Header().Set("Content-Type", "application/json")
		c.JSON(200, content)
	}
}
