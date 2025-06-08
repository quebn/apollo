package main

import (
	"database/sql"
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
	sql_statement := `
	create table if not exists musics (id integer not null primary key, title text not null, path text not null);
	`
	_, err = db.Exec(sql_statement)
	if err != nil {
		log.Printf("%q: %s\n", err, sql_statement)
		return db, err
	}
	return db, nil
}
