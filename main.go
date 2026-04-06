package main

/**
curl upload testing command: curl -X POST http://localhost:8080/upload -F "file=@test.txt"
curl retrieve testing command: curl http://localhost:8080/files/test.txt
curl retrieve stored files command: curl http://localhost:8080/files
*/

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

const uploadDir = "./uploads"
const maxFileSize = 50 << 2

func main() {
	Testing()
	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic("Failed to create uploads directory: " + err.Error())
	}

	r := gin.Default()

	// Middleware to log requests
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.MaxMultipartMemory = maxFileSize

	// Routes
	r.GET("/", index)
	r.POST("/upload", uploadFile)
	r.GET("/files/get_files/:filename/:meta", getFile)
	r.GET("/files/listfiles", listFiles)
	r.GET("/files/delete/:filename", deleteFile)

	fmt.Println("\n File API running on http://localhost:8080")
	err := r.Run(":8080")
	if err != nil {
		fmt.Println("An error has occurred when attempting to launch the server. Although you probably guessed that", err)
		return
	}
}
