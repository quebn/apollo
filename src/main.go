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
	"strconv"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/sevlyar/go-daemon"
)

type MusicManager struct {
	playlist []string
	loop bool
	playing bool
	current int
	music_dir string
	toggle chan bool
	volume chan float64
}

type Daemon struct {
	context *daemon.Context
	network string
	rpc_port string
}


func main() {
	cmd, args := parse_args()

	dmon := Daemon{ network: "tcp", rpc_port: ":42069" }
	if (cmd != "start") {
		handle_daemon(&dmon, cmd, args)
		return
	}
	dmon.context = &daemon.Context {
		PidFileName: "/tmp/apollo.pid",
		PidFilePerm: 0644,
		LogFileName: "/tmp/apollo.log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
	}
	process, err := dmon.context.Reborn()
	if err != nil {
		return
	}

	if process != nil {
		fmt.Printf("Starting Apollo...\n")
		return
	}

	defer dmon.context.Release()
	playlist := []string{}
	if args != nil || len(args) > 0 {
		for _, v := range args {
			playlist = append(playlist, v.(string))
		}
	}
	manager := MusicManager{
		playlist: playlist,
		loop: true,
		current: 0,
		playing: false,
		music_dir: "./public",
		toggle: make(chan bool),
		volume: make(chan float64),
	}
	start_rpc(&dmon, &manager)
	// start_musicplayer()
	// start_http()
}

func handle_daemon(d *Daemon, cmd string, args []any) {
	client, err := rpc.Dial(d.network, d.rpc_port)
	if err != nil {
		fmt.Printf("Apollo daemon is not active...\n")
		return
	}

	var reply string
	switch cmd {
	case "play":
		err = client.Call("MusicManager.Play", "", &reply)
		fmt.Printf("Playing Song...\n")
	case "toggle":
		err = client.Call("MusicManager.Toggle", "", &reply)
	case "kill":
		err = client.Call("Daemon.Kill", "", &reply)
	case "vol":
		value := args[0].(float64)
		err = client.Call("MusicManager.Volume", value, &reply)
	default:
		fmt.Printf("No Handles implemented for command: %s\n", cmd)
		return
	}

	if err != nil {
		return
	}
	fmt.Printf("Apollo: %s\n", reply)
}

func (d *Daemon) Kill(args string, reply *string) error {
	d.context.Release()
	os.Exit(0)
	return nil
}

func (m *MusicManager) Play(args string, reply *string) error {
	if !m.playing {
		*reply = "Song is Playing...."
		go m.play_playlist()
	} else {
		*reply = "Already Playing song...."
	}
	return nil
}

func (m *MusicManager) Toggle(args string, reply *string) error {
	if m.playing {
		*reply = "Song Toggled!"
		m.toggle <-true
		return nil
	}
	*reply = "No Song Playing..."
	return nil
}

func (m *MusicManager) Volume(args float64, reply *string) error {
	if m.playing {
		*reply = fmt.Sprintf("Setting volume to %g\n", args)
		m.volume <-args
		return nil
	}
	*reply = "No Song Playing..."
	return nil
}


func start_rpc(d *Daemon, m *MusicManager) {
	rpc.RegisterName("MusicManager", m)
	rpc.RegisterName("Daemon", d)
	listener, err := net.Listen(d.network, d.rpc_port)
	if err != nil {
		panic(err)
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
func (m *MusicManager) play_playlist() {
	if len(m.playlist) == 0 {
		songs, err := try_getpath(m.music_dir)
		if err != nil {
			return
		}
		m.playlist = songs
	}
	m.playing = true
	for m.current < len(m.playlist) {
		file_path := m.playlist[m.current]

		file, err := os.Open(file_path)
		if err != nil {
			fmt.Printf("Failed to open file %s: not a valid format\n", file_path)
			panic(err)
		}
		defer file.Close()

		streamer, format, err := vorbis.Decode(file)
		if err != nil {
			fmt.Printf("Failed to decode file %s: not a valid format\n", file_path)
			panic(err)
		}
		defer streamer.Close()

		fmt.Printf("Now Playing: %s\n", file_path)
		m.play_song(streamer, format)
		m.current++
		if m.loop && len(m.playlist) == m.current {
			m.current = 0
		}
	}
	m.playing = false
}

func (m *MusicManager) play_song(s beep.StreamSeekCloser, format beep.Format) {
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	ctrl := &beep.Ctrl{Streamer: beep.Loop(1, s), Paused: false}
	vol := &effects.Volume{
		Streamer: ctrl,
		Base: 2,
		Volume: 0,
		Silent: false,
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(vol, beep.Callback(func(){
		done <-true
	})))
	for {
		select {
		case <-done:
			return
		case <-m.toggle:
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
		case volume :=<-m.volume:
			speaker.Lock()
			vol.Volume += volume
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
	dirfs := os.DirFS("./public")
	files, err = fs.Glob(dirfs, fmt.Sprintf("%s.ogg", name))
	// TODO: support other formats (mp3, wav, and etc..)
	if err != nil || files == nil {
		fmt.Printf("No file with name of %s found!\n", name)
		return files, errors.New("File with name of %s does not exist!\n")
	}
	return files, nil
}

// playlist should be additional args
func parse_args() (cmd string, args []any) {
	if len(os.Args) == 1 {
		return "start", args
	}
	arg := os.Args[1]
	switch arg {
	case "play", "toggle", "next", "prev", "stop", "list", "kill":
		return arg, args
	case "vol":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr ,"ERROR: volume argument value required\n")
			fmt.Fprintf(os.Stderr ,"USAGE: apollo vol [VALUE] \n")
			os.Exit(1)
		}
		cmd = arg
		arg = os.Args[2]
		args = make([]any, 1)
		var err error
		args[0], err = strconv.ParseFloat(arg, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr ,"ERROR: invalid volume value '%s'\n", arg)
			fmt.Fprintf(os.Stderr ,"USAGE: apollo vol [VALUE] \n")
			os.Exit(1)
		}
		return cmd, args
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
		args = make([]any, len(list))
		for i, v := range list {
			args[i] = v
		}
	}
	return "start", args
}
