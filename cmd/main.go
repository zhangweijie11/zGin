package main

import (
	"fmt"
	"github.com/zhangweijie11/zGin"
	"net/http"
)

type Person struct {
	Name string `json:"name" binding:"required"`
	Age  int    `json:"age"`
}

func main() {
	router := gin.Default()
	router.GET("/user/:name", func(c *gin.Context) {
		return
		//name := c.Param("name")
		//c.String(http.StatusOK, "Hello %s", name)
	})
	router.POST("/user/age", func(c *gin.Context) {
		var person Person
		err := c.ShouldBindJSON(&person)
		fmt.Println("------------>", err)
		c.String(http.StatusOK, "Hello World")
		return
		//name := c.Param("name")

	})
	//router.Run(":8080")
	router.GET("/user/age1", func(c *gin.Context) {
		return
		//name := c.Param("name")
		//c.String(http.StatusOK, "Hello %s", name)
	})
	router.POST("/user/age12", func(c *gin.Context) {
		return
		//name := c.Param("name")
		//c.String(http.StatusOK, "Hello %s", name)
	})
	router.POST("/admin/age12", func(c *gin.Context) {
		return
		//name := c.Param("name")
		//c.String(http.StatusOK, "Hello %s", name)
	})
	router.Run()
}
