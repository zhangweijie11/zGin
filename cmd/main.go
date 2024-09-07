package main

import (
	"fmt"
	"github.com/zhangweijie11/zGin"
)

func main() {
	router := gin.Default()
	router.GET("/user/:name", func(c *gin.Context) {
		return
		//name := c.Param("name")
		//c.String(http.StatusOK, "Hello %s", name)
	})
	router.GET("/user/age", func(c *gin.Context) {
		fmt.Println("------------>", "1111")
		return
		//name := c.Param("name")
		//c.String(http.StatusOK, "Hello %s", name)
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
