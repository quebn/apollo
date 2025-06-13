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
	result, err := db.Query("select * from musics;")
	if err != nil {
		log.Fatal(err)
		return musics
	}
	for result.Next() {
		song := Music{}
		err = result.Scan(&song.id, &song.title, &song.path)
		if err != nil {
			continue
		}
		musics = append(musics, song)
	}
	return musics
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
		return fmt.Errorf("Error: inserting %s to db: %v", values, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
		return nil
	}
	fmt.Printf("Insert to db success with %d row(s) affected\n", count)
	return nil
}

func clean_musics(db *sql.DB) uint {
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
	select m.*
	from musics m
	join playlist_songs ps on m.id = ps.music_id
	join playlists p on ps.playlist_id = p.id
	where p.name = '%s';`, name)
	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("Error getting musics from database:%v\n", err)
		return playlist, err
	}
	playlist.name = name
	for rows.Next() {
		song := Music{}
		err = rows.Scan(&song.id ,&song.id, &song.path)
		if err != nil {
			continue
		}
		playlist.songs = append(playlist.songs, song)
	}
	return playlist, nil
}

func exists(db *sql.DB, table string, where string) bool {
	query := fmt.Sprintf("select exists (select 1 from %s where %s);", table, where)
	row := db.QueryRow(query)
	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking on %s table where %s: %v\n", table, where, err)
		return false
	}
	return exists
}

func get_song(db *sql.DB ,title string) (Music, error) {
	song := Music{}
	query := fmt.Sprintf("select * from musics where title = '%s' limit 1;", title)
	row := db.QueryRow(query)
	err := row.Scan(&song.id, &song.title, &song.path)
	if err != nil {
		return song, fmt.Errorf("Cannot get song from db: %v", err)
	}
	return song, nil
}

func sync_musics(db *sql.DB, dirpath string, fallback string) (msg string, err error)  {
	msg = ""
	if dirpath == "" {
		file, err := os.Stat(fallback)
		if err != nil || !file.IsDir() {
			return msg, fmt.Errorf("Default Dir: %s is not a valid directory path: %v", fallback,  err)
		}
		msg = "Syncing database to default directory"
		return msg, register_dir(db, fallback)
	}
	info, err := os.Stat(dirpath)
	if err != nil || !info.IsDir() {
		msg = fmt.Sprintf("Invalid argument '%s': not a directory path", dirpath)
		return msg, fmt.Errorf("%s", msg)
	}
	msg = fmt.Sprintf("Syncing database to '%s'", dirpath)
	return msg, register_dir(db, dirpath)
}

func list_musics(db *sql.DB) string {
	msg := ""
	songs := get_all_songs(db)
	if len(songs) == 0 {
		msg = "No songs found in database"
	} else {
		msg = "Listing Database Records"
		for _, song := range songs {
			msg = fmt.Sprintf("%s\n%d: %s -> %s", msg, song.id, song.title, song.path)
		}
	}
	return msg
}

func create_playlist(db *sql.DB, name string) (string, error) {
	if exists(db, "playlists", fmt.Sprintf("name = %s", name)) {
		return fmt.Sprintf("Playlist: '%s' already exists", name), nil
	}
	_, err := db.Exec("insert into playlists(name) values (?);", name)
	if err != nil {
		return fmt.Sprintf("ERROR on inserting: %v", err), err
	}
	return fmt.Sprintf("Successfully created playlist '%s'!", name), nil
}

func delete_playlist(db *sql.DB, name string) (string, error) {
	result, err := db.Exec("delete from playlists where name = ?;", name)
	if err != nil {
		return fmt.Sprintf("Error deleting musics from database:%v", err), err
	}
	rows_affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Sprintf("Error getting rows affected:%v", err), err
	}
	if rows_affected == 0 {
		return fmt.Sprintf("No playlist with '%s' in database!", name), nil
	}
	return fmt.Sprintf("Successfully deleted playlist '%s'!", name), nil
}

func list_playlist(db *sql.DB) (string, error) {
	msg := ""
	query := `
	select playlists.*, count(playlist_songs.music_id) as song_count
	from playlists
	left join playlist_songs on playlists.id = playlist_songs.playlist_id
	group by playlists.id;
	`
	result, err := db.Query(query)
	if err != nil {
		return msg, fmt.Errorf("ERROR: query error of playlists %v", err)
	}
	for result.Next() {
		var id int
		var count int
		var name string
		err = result.Scan(&id, &name, &count)
		if err != nil {
			continue
		}
		msg = fmt.Sprintf("%s\n[%d] %s with %d song(s)", msg, id, name, count)
	}
	if msg == "" {
		msg = "No playlists found"
	}
	return msg, nil
}
