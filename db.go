package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

// A testing/utility function for making changes to a sql db
func dbFunction() {
	db, err := sql.Open("sqlite3", fileSystemBaseDir + "/master.db")
	if err != nil {
		Fatal(err.Error())
	}
	defer db.Close()
// 	sqlStmt := `
//     CREATE TABLE files (
//     id INTEGER PRIMARY KEY AUTOINCREMENT,
//     filename TEXT NOT NULL,
//     acts_id TEXT,
//     sha256_hash TEXT,
//     created_at DATETIME DEFAULT CURRENT_TIMESTAMP
// );
//     `
// 	_, err = db.Exec(sqlStmt)
// 	if err != nil {
// 		Fatal(err.Error())
// 	}
// 	Debug("Table 'users' created successfully")
}

// db, err := sql.Open("sqlite3", "./test.db")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer db.Close()
// 	_, err = db.Exec("INSERT INTO users(name) VALUES(?)", "John Doe")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	log.Println("New user inserted successfully")
// import (
// 	"database/sql"
// 	"log"
// 	_ "github.com/mattn/go-sqlite3"
// )
//
// func main() {
// 	db, err := sql.Open("sqlite3", "./test.db")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer db.Close()
// 	sqlStmt := `
//     CREATE TABLE IF NOT EXISTS users (
//         id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
//         name TEXT
//     );
//     `
// 	_, err = db.Exec(sqlStmt)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	log.Println("Table 'users' created successfully")
// }
