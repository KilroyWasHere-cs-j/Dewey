package main

import (
	"time"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func InitDB() error {
	var err error
	db, err = sql.Open("sqlite3", fileSystemBaseDir+"/master.db")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)

	return db.Ping()
}

func createNewFileRecord(filename string, acts_id string, sha256_hash string) {
	tx, err := db.Begin()
	if err != nil {
		Warn(err.Error())
		return
	}

	now := time.Now()

// INSERT INTO files (filename, acts_id, sha256_hash, created_at)
// VALUES ('insert_test.txt', 'ACTS-003','sha256', current_date);
	_, err = tx.Exec("INSERT INTO files (filename, acts_id, sha256_hash, created_at) VALUES (?, ?, ?, ?)", filename, acts_id, sha256_hash, now)
	if err != nil {
		tx.Rollback()
		Warn(err.Error())
		return
	}

	tx.Commit()

	return 
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
