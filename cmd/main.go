package main

import (
	"fmt"
	"github.com/zhangweijie11/zGin"
)

func main() {
	router := gin.Default()
	fmt.Println("------------>", router)
	//router.GET("/user/:name", func(c *gin.Context) {
	//	name := c.Param("name")
	//	c.String(http.StatusOK, "Hello %s", name)
	//})
	//router.Run(":8080")
}
