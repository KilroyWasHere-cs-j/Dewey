package main

/**
curl upload testing command: curl -X POST http://localhost:8080/upload -F "file=@test.txt"
curl retrieve testing command: curl http://localhost:8080/files/test.txt
curl retrieve stored files command: curl http://localhost:8080/files
*/


/*
	Error code index
	0 = clean exit no error
	3 = directory error
	5 = server error
*/

import (
	"os"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	InitLogger("logs", "app")
	defer logger.Close()

	fileSystemInit()

	Debug("Server start")
	r := gin.Default()

	// Middleware to log requests
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.MaxMultipartMemory = maxFileSize

	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"code": "PAGE_NO_FOUND", "message" : "Page not found"})
	})

	r.LoadHTMLGlob(htmlTemplatesDir + "/*")
	// Routes
	r.GET("/", index)
	r.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "adminportal.html", nil)
	})
	r.GET("/settings", func(c *gin.Context) {
		c.HTML(http.StatusOK, "settings.html", nil)
	})
	r.POST("/upload", uploadFile)
	r.GET("/files/get_files/:filename/:meta", getFile)
	r.GET("/files/listfiles", listFiles)
	r.GET("/files/delete/:filename", deleteFile)

	Debug("Server is running at port: " + portNumber)	
	err := r.Run(":" + portNumber)
	if err != nil {
		Fatal(err.Error())	
		os.Exit(5)
	}
}
