package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

func createDB() {
	Debug("Creating database")
	db, err := sql.Open("sqlite3", fileSystemBaseDir + "/master.db")
	if err != nil {
		Fatal(err.Error())
	}
	defer db.Close()
}
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
