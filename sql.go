package main

/*
	https://www.twilio.com/en-us/blog/developers/community/use-sqlite-go
*/

import (
	"database/sql"
	"log"
	_ "github.com/mattn/go-sqlite3"
)

// DO NOT run this function unless you are Gabe
func dbInit() {
	db, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	sqlStmt := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        name TEXT
    );
    `
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Table 'users' created successfully")
}

// Open db connection

// Close db connection

// Insert file record

// Delete file record (just mark as deleted not for real)


