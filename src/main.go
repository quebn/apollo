package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/sevlyar/go-daemon"
)

var music_dir = "./public"
var current = 0
var loop = true
var network = "tcp"
var rpc_port = ":42069"
var context *daemon.Context
var playlist []string
var playing bool

type Apollo struct{}

func main() {
	cmd := parse_args()
	if (cmd != "start") {
		handle_daemon(cmd)
		return
	}
	context = &daemon.Context{
		PidFileName: "/tmp/apollo.pid",
		PidFilePerm: 0644,
		LogFileName: "/tmp/apollo.log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
	}
	process, err := context.Reborn()
	if err != nil {
		return
	}

	if process != nil {
		fmt.Printf("Starting Apollo...\n")
		return
	}

	defer context.Release()
	start_rpc()
	// start_musicplayer()
	// start_http()
}

func handle_daemon(cmd string) {
	client, err := rpc.Dial(network, rpc_port)
	if err != nil {
		fmt.Printf("Apollo daemon is not active...\n")
		return
	}

	var reply string
	switch cmd {
	case "play":
		err = client.Call("Apollo.Play", "", &reply)
	case "toggle":
		err = client.Call("Apollo.Toggle", "", &reply)
	case "kill":
		err = client.Call("Apollo.Kill", "", &reply)
	default:
		fmt.Printf("No Handles implemented for command: %s\n", cmd)
		return
	}

	fmt.Printf("Playing Song...\n")
	if err != nil {
		return
	}
	fmt.Printf("Apollo: %s\n", reply)
}

func (a *Apollo) Kill(args string, reply *string) error {
	context.Release()
	os.Exit(0)
	return nil
}

func (a *Apollo) Play(args string, reply *string) error {
	if !playing {
		*reply = "Song is Playing...."
		go play_music()
	} else {
		*reply = "Already Playing song...."
	}
	return nil
}

func (a *Apollo) Toggle(args string, reply *string) error {
	if playing {
		*reply = "Song Toggled!"
		toggle <- true
		return nil
	}
	*reply = "No Song Playing..."
	return nil
}


func start_rpc() {
	rpc.Register(new(Apollo))
	listener, err := net.Listen(network, rpc_port)
	if err != nil {
		fmt.Printf("ERROR listening to tcp with error of:%v\n", err)
		return
	}
	fmt.Printf("Apollo Started!\n")
	for {
		conn, err:= listener.Accept()
		if err != nil {
			continue
		}
		go rpc.ServeConn(conn)
	}
}

// TODO: support other formats
func play_music() {
	if len(playlist) == 0 {
		songs, err := try_getpath(music_dir)
		if err != nil {
			return
		}
		playlist = songs
	}
	playing = true
	for current < len(playlist) {
		file_path := playlist[current]

		file, err := os.Open(file_path)
		if err != nil {
			fmt.Printf("Failed to open ogg file %s\n", file_path)
			os.Exit(2)
		}
		defer file.Close()

		streamer, format, err := vorbis.Decode(file)
		if err != nil {
			fmt.Printf("Failed to decode %s\n", file_path)
			os.Exit(2)
		}
		defer streamer.Close()

		fmt.Printf("Now Playing: %s\n", file_path)
		play_song(streamer, format)
		current++
		if loop && len(playlist) == current {
			current = 0
		}
	}
	playing = false
}

var toggle = make(chan bool)
func play_song(s beep.StreamSeekCloser, format beep.Format) {
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	ctrl := &beep.Ctrl{Streamer: beep.Loop(1, s), Paused: false}

	// speaker.Play(ctrl)
	done := make(chan bool)
	speaker.Play(beep.Seq(ctrl, beep.Callback(func(){
		done <- true
	})))
	// <- done
	for {
		select {
		case <-done:
			return
		case <-toggle:
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
		}
	}
}

func try_getpath(name string) ([]string, error) {
	files := []string{}
	file, err := os.Stat(name)
	if err != nil {
		return files, err
	}
	fmt.Printf("Checking if dir or file: %s.\n", name)
	if !file.IsDir() {
		files = append(files, name)
		fmt.Printf("Found file %s\n", name)
		return files, nil
	} else {
		dirfs := os.DirFS(strings.TrimRight(name, "/"))
		// TODO: support other formats (mp3, wav, and etc..)
		songs, err := fs.Glob(dirfs, "*.ogg")
		if err == nil && len(songs) > 0 {
			fmt.Printf("Found dir %s\n", name)
			for _, song := range songs {
				fmt.Printf("Adding to files: %s\n", fmt.Sprintf("%s/%s", name, song))
				files = append(files, fmt.Sprintf("%s/%s", name, song))
			}
			return files, nil
		}
	}
	// TODO: make this option use the database the get the name of the file and
	// get its loc where the location is the primary key
	fmt.Printf("Checking if title\n")
	dirfs := os.DirFS(music_dir)
	files, err = fs.Glob(dirfs, fmt.Sprintf("%s.ogg", name))
	// TODO: support other formats (mp3, wav, and etc..)
	if err != nil || files == nil {
		fmt.Printf("No file with name of %s found!\n", name)
		return files, errors.New("File with name of %s does not exist!\n")
	}
	return files, nil
}

func parse_args() string{
	arg := os.Args[1]
	switch arg {
	case "play", "toggle", "next", "prev", "stop", "list", "kill":
		return arg
	case "help":
		fmt.Print("NOT IMPLEMENTED\n")
		os.Exit(0)
	default:
		list, err := try_getpath(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr ,"ERROR: %s is not a valid song argument or command\n", arg)
			fmt.Fprintf(os.Stderr ,"USAGE: apollo [COMMAND | FILEPATH | DIRPATH | TITLE] \n")
			os.Exit(1)
		}
		playlist = list
	}
	return "start"
}
