package main

import (
	"fmt"
	"net/rpc"
)

func handle_daemon(d *Daemon, cmd string, args []any) {
	client, err := rpc.Dial(d.network, d.config.RpcPort)
	if err != nil {
		handle_offline(cmd, args, *d.config)
		return
	}

	var reply string
	switch cmd {
	case "sync":
		dirpath := ""
		if len(args) > 0 {
			dirpath = args[0].(string)
		}
		err = client.Call("MusicManager.Sync", dirpath, &reply)
	case "play":
		name := ""
		if len(args) > 0 {
			name = args[0].(string)
			db, err := get_db()
			if err != nil {
				fmt.Printf("Apollo: error getting db: %v!\n", err)
				return
			}
			defer db.Close()
			if !exists(db, "playlists", fmt.Sprintf("name = '%s'", name)) {
				fmt.Printf("Apollo: playlist '%s' does not exist!\n", name)
				return
			}
		}
		err = client.Call("MusicManager.Play", name, &reply)
	case "stop":
		err = client.Call("MusicManager.Stop", "", &reply)
	case "toggle":
		err = client.Call("MusicManager.Toggle", "", &reply)
	case "next":
		err = client.Call("MusicManager.Next", "", &reply)
	case "list":
		err = client.Call("MusicManager.List", "", &reply)
	case "playlist":
		err = client.Call("MusicManager.Playlist", "", &reply)
	case "prev":
		err = client.Call("MusicManager.Previous", "", &reply)
	case "clean":
		err = client.Call("MusicManager.Clean", "", &reply)
	case "vol":
		value := args[0].(float64)
		err = client.Call("MusicManager.Volume", value, &reply)
	case "create":
		name := args[0].(string)
		err = client.Call("MusicManager.Create", name, &reply)
	case "delete":
		name := args[0].(string)
		err = client.Call("MusicManager.Delete", name, &reply)
	case "playlists":
		err = client.Call("MusicManager.Playlists", "", &reply)
	case "add":
		path := ""
		if len(args) > 0 {
			path = args[0].(string)
		}
		err = client.Call("MusicManager.AddMusic", path, &reply)
	case "kill":
		err = client.Call("Daemon.Kill", "", &reply)
		fmt.Printf("Apollo Daemon killed\n")
		return
	default:
		fmt.Printf("No Handles implemented for command: %s\n", cmd)
		return
	}

	if err != nil {
		return
	}
	fmt.Printf("Apollo: %s\n", reply)
}

func handle_offline(cmd string, args []any,  config Config) {
	db, err := get_db()
	if err != nil {
		fmt.Printf("Unable to get the database: %v\n", err)
		return
	}
	defer db.Close()
	var reply string
	switch cmd {
	case "sync":
		// check and set args
		arg := ""
		if len(args) > 0 {
			arg = args[0].(string)
		}
		reply, err = sync_musics(db, arg, config.MusicDir)
	case "list":
		reply = list_musics(db)
	case "clean":
		changes := clean_musics(db)
		reply = fmt.Sprintf("Cleaned %d item(s) in the database", changes)
	case "create":
		name := args[0].(string)
		reply, err = create_playlist(db, name)
	case "delete":
		name := args[0].(string)
		reply, err = delete_playlist(db, name)
	case "playlists":
		reply, err = list_playlist(db)
	default:
		reply = "Daemon is not active..."
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Apollo: %s\n", reply)
}
