package main

import (
	"example/controllers"
	"example/models"
	"fmt"
	"github.com/gin-gonic/gin"
)

func main() {

	// Configure the mongodm connection
	dbConfig := &models.Config{
		DatabaseHosts: []string{"mongo:27017"},
		DatabaseName:  controllers.DBName,
	}

	// Connect and check for error
	db, err := models.Connect(dbConfig)
	if err != nil {
		fmt.Println("Failed to establish connection to Mongo server:", err)
	} else {
		controllers.Database = db
	}

	uc := controllers.NewUserController(db)
	r := gin.New()
	// enable Logging
	r.Use(gin.Logger())
	v1 := r.Group("/api/v1")
	{
		v1.POST("/user", uc.Create)
		v1.GET("/user", uc.UsersList)
		v1.GET("/user/:id", uc.GetUser)
		v1.PUT("/user/:id", uc.UpdateUser)
	}

	r.Run()
}
