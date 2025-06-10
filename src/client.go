package main

import (
	"fmt"
	"net/rpc"
)

func handle_daemon(d *Daemon, cmd string, args []any) {
	client, err := rpc.Dial(d.network, d.config.RpcPort)
	if err != nil {
		fmt.Printf("Apollo daemon is not active...\n")
		return
	}

	var reply string
	switch cmd {
	case "sync":
		arg := ""
		if len(args) > 0 {
			arg = args[0].(string)
		}
		err = client.Call("MusicManager.Sync", arg, &reply)
	case "play":
		err = client.Call("MusicManager.Play", "", &reply)
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
	case "kill":
		err = client.Call("Daemon.Kill", "", &reply)
	default:
		fmt.Printf("No Handles implemented for command: %s\n", cmd)
		return
	}

	if err != nil {
		return
	}
	fmt.Printf("Apollo: %s\n", reply)
}

