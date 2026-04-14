package main

import (
	"os"
)

// PDF files should be treated as seperate files for each page with there own records

func loadFilters() {

}

func fileSystemInit() {
	// Create caching directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		Fatal(err.Error())
		os.Exit(3)
	}

	// Create store directory if it doesn't exist
	if err := os.MkdirAll(fileSystemBaseDir , 0755); err != nil {
		Fatal(err.Error())
		os.Exit(3)
	}
}

func idAndSort(path string){
	//	barcodeText := scanBarCode(path)

	// Determine where the file needs to go
	// Create and store db entry
	// Store file bytes and dn entry route
}

func searchAndReturn(name string){
	// Check cache first
	// Search for a given file by name or other metadata
	// Return file and metadata if needed
}

func changeMeta(){
	// Pull metadata from SQL
	// Update records according to the uploaded meta
}

func setFileStatus(){
	// searchAndReturn()

	// Search up file retrive it's path
	// Flip the deleted flag
}
