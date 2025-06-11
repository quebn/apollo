package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	pragma foreign_keys = on;
	create table if not exists musics (
		id integer not null primary key,
		title text not null,
		path text not null
	);
	create table if not exists playlists (
		id integer not null primary key,
		name text not null
	);
	create table if not exists playlist_songs (
		id integer not null,
		playlist_id integer not null,
		music_id integer not null,
		foreign key (playlist_id) references playlists(id) on delete cascade,
		foreign key (music_id) references musics(id) on delete cascade
	);
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("%q: %s\n", err, query)
		return db, err
	}
	return db, nil
}

func get_all_songs(db *sql.DB) []Music {
	musics := []Music{}
	result, err := db.Query("select title, path from musics;")
	if err != nil {
		log.Fatal(err)
		return musics
	}
	for result.Next() {
		var title string
		var path string
		err = result.Scan(&title, &path)
		musics = append(musics, Music{title: title, path: path})
	}
	return musics
}

func register(db *sql.DB, title string, path string) error {
	// check if title or path already exist.
	row := db.QueryRow("select exists (select 1 from musics where title = ? or path = ?);", title, path)
	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		return fmt.Errorf("ERROR on %s and path of %s: %v\n", title, path, err)
	}
	if exists {
		fmt.Printf("Music with title of %s or path of %s already registered\n", title, path)
		return nil
	}
	result, err := db.Exec("insert into musics(title, path) values (? , ?);",  title, path)
	if err != nil {
		return fmt.Errorf("ERROR on inserting: %v\n", err)
	}
	rows_affected, err := result.RowsAffected()
	fmt.Printf("Inserted to database with %d row(s) affected!\n", rows_affected)
	return nil
}

func register_dir(db *sql.DB, dirpath string) error {
	songs, err := get_songs_from_dir(dirpath)
	if err != nil {
		return fmt.Errorf("Error getting songs from %s: %v\n", dirpath, err)
	}
	rows, err := db.Query("select path from musics;")
	if err != nil {
		return fmt.Errorf("Error Querying songs from db: %v\n", err)
	}
	exists := []string{}
	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			continue
		}
		exists = append(exists, path)
	}

	new_songs := []string{}
	for _, song := range songs {
		path := song.path
		if !slices.Contains(exists, path) {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(info.Name(), ".ogg")
			value := fmt.Sprintf("('%s', '%s')", name, path)
			new_songs = append(new_songs, value)
		}
	}
	if len(new_songs) == 0 {
		fmt.Printf("No new values to insert\n")
		return nil
	}
	values := strings.Join(new_songs, ",")
	result, err := db.Exec(fmt.Sprintf("insert into musics(title, path) values %s;", values))
	if err != nil {
		return fmt.Errorf("Error: inserting %s to db: %v\n", values, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
		return nil
	}
	fmt.Printf("Insert to db success with %d row(s) affected\n", count)
	return nil
}

func clean_database(db *sql.DB) uint {
	paths := []string{}
	rows, err := db.Query("select path from musics;")
	if err != nil {
		fmt.Printf("Error getting musics from database:%v\n", err)
		return 0
	}
	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		paths = append(paths, path)
	}
	invalid_paths := []string{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			continue
		}
		invalid_paths = append(invalid_paths, path)
	}
	values := strings.Join(invalid_paths, ",")
	result, err := db.Exec(fmt.Sprintf("delete from musics where path in (%s);", values))
	if err != nil {
		fmt.Printf("Error deleting musics from database:%v\n", err)
		return 0
	}
	rows_affected, err := result.RowsAffected()
	if err != nil {
		fmt.Printf("Error getting rows affected:%v\n", err)
		return 0
	}
	return uint(rows_affected)
}

func get_playlist(db *sql.DB, name string) (Playlist, error) {
	playlist := Playlist{
		name: "",
		songs: []Music{},
	}
	query := fmt.Sprintf(`
	select m.title, m.path
	from musics m
	join playlist_songs ps on m.id = ps.music_id
	join playlist p on ps.playlist_id = p.id
	where p.name = '%s';`, name)
	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("Error getting musics from database:%v\n", err)
		return playlist, err
	}
	playlist.name = name
	for rows.Next() {
		var title string
		var path string
		err = rows.Scan(&name, &path)
		if err != nil {
			continue
		}
		playlist.songs = append(playlist.songs, Music{title, path})
	}
	return playlist, nil
}
