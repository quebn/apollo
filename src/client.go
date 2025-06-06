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
	case "play":
		err = client.Call("MusicManager.Play", "", &reply)
		fmt.Printf("Playing Song...\n")
	case "stop":
		err = client.Call("MusicManager.Stop", "", &reply)
	case "toggle":
		err = client.Call("MusicManager.Toggle", "", &reply)
	case "next":
		err = client.Call("MusicManager.Next", "", &reply)
	case "list":
		err = client.Call("MusicManager.List", "", &reply)
	case "prev":
		err = client.Call("MusicManager.Previous", "", &reply)
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

