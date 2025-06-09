package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func get_db() (*sql.DB, error) {
	data_dirpath := os.Getenv("HOME") + "/.local/share/apollo"
	db_filepath := filepath.Join(data_dirpath, "apollo.db")
	get_dir(data_dirpath)

	db, err := sql.Open("sqlite3", db_filepath)
	if err != nil {
		return db, err
	}
	query := `
	create table if not exists musics (id integer not null primary key, title text not null, path text not null);
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("%q: %s\n", err, query)
		return db, err
	}
	return db, nil
}

func register_song(db *sql.DB, title string, path string) error {
	// check if title or path already exist.
	row := db.QueryRow("select exists (select 1 from musics where title = ? or path = ?);", title, path)
	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		panic(err)
	}
	if exists {
		return errors.New(fmt.Sprintf("Music with title of %s or path of %s already registered\n", title, path))
	}
	result, err := db.Exec("insert into musics(title, path) values (? , ?);",  title, path)
	if err != nil {
		log.Fatal(err)
	}
	rows_affected, err := result.RowsAffected()
	fmt.Printf("Inserted to database with %d row(s) affected!\n", rows_affected)
	return nil
}
