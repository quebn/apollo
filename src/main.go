package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/sevlyar/go-daemon"
)

type MusicManager struct {
	playlist []string
	playing bool
	current int
	paused bool
	config *Config

	// channels
	toggle chan bool
	volume chan float64
	done chan bool
}

type Daemon struct {
	context *daemon.Context
	network string
	config *Config
}


func main() {
	config := get_config()
	cmd, args := parse_args(config)

	dmon := Daemon{ network: "tcp", config: config }
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

	if len(playlist) == 0 {
		playlist, err = get_songs_from_dir(config.MusicDir)
		if err != nil {
			fmt.Printf("WARN: cannot get music files from %s\n", config.MusicDir)
		}
	}

	manager := MusicManager{
		playlist: playlist,
		config: config,
		current: 0,
		playing: false,
		toggle: make(chan bool),
		volume: make(chan float64),
		done: make(chan bool),
	}

	db, err := get_db()
	defer db.Close()

	start_rpc(&dmon, &manager)
	// start_musicplayer()
	// start_http()
}

func (d *Daemon) Kill(args string, reply *string) error {
	*reply = "Daemon Killed"
	d.context.Release()
	save_config(d.config)
	os.Exit(0)
	return nil
}

func (m *MusicManager) Play(args string, reply *string) error {
	if !m.playing {
		if len(m.playlist) == 0 {
			*reply = "No songs in the playlist"
			return nil
		}
		*reply = "Playing Song: " + m.playlist[m.current]
		go m.play_playlist()
	} else {
		if m.paused == true {
			m.toggle <-false
			*reply = "Already Playing song and unpausing instead...."
		}
		*reply = "Already Playing song...."
	}
	return nil
}

func (m *MusicManager) Stop(args string, reply *string) error {
	if m.playing {
		m.playing = false
		m.toggle <-true
		m.done <-true
		*reply = fmt.Sprintf("Stopping at index: %d", m.current)
	} else {
		*reply = "Apollo is not playing anything..."
	}
	return nil
}

func (m *MusicManager) Previous(args string, reply *string) error {
	if len(m.playlist) == 0 {
		return errors.New("No songs in playlist")
	}
	if m.current == 0 {
		m.current = len(m.playlist) - 1
	} else {
		m.current--
	}
	*reply = fmt.Sprintf("Previous with index: %d\b", m.current)
	if m.playing {
		m.playing = false
		m.toggle <-true
		m.done <-true
		go m.play_playlist()
	}
	return nil

}

func (m *MusicManager) List(args string, reply *string) error {
	if len(m.playlist) == 0 {
		*reply = "No Songs in Playlist"
	} else {
		*reply = "Playlist Songs:\n"
		for i,v := range m.playlist {
			row := fmt.Sprintf("[%d]: %s\n", i, v)
			*reply = *reply + row
		}
	}
	return nil
}

func (m *MusicManager) Next(args string, reply *string) error {
	if len(m.playlist) == 0 {
		return errors.New("No songs in playlist")
	}
	if m.playing {
		m.toggle <-true
		m.done <-true
		m.toggle <-false
		*reply = "Going next"
	} else{
		m.current++
		if len(m.playlist) == m.current {
			m.current = 0
		}
		*reply = fmt.Sprintf("Next with index: %d\b", m.current)
	}
	return nil
}

func (m *MusicManager) Toggle(args string, reply *string) error {
	if m.playing {
		*reply = "Song Toggled!"
		m.toggle <-!m.paused
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
	listener, err := net.Listen(d.network, d.config.RpcPort)
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
		fmt.Printf("No songs in playlist\n")
		return
	}
	m.paused = false
	m.playing = true
	fmt.Printf("Playlist Playing...!\n")
	for m.playing && m.current < len(m.playlist) {
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
		if m.playing {
			fmt.Printf("Incrementing current index: %d -> %d\n", m.current, m.current+1)
			m.current++
			if m.config.Loop && len(m.playlist) == m.current {
				m.current = 0
			}
		}
	}
	m.playing = false
	fmt.Printf("Playlist stopped!\n")
}

func (m *MusicManager) play_song(s beep.StreamSeekCloser, format beep.Format) {
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	ctrl := &beep.Ctrl{Streamer: beep.Loop(1, s), Paused: m.paused}
	vol := &effects.Volume{
		Streamer: ctrl,
		Base: 2,
		Volume: 0,
		Silent: false,
	}
	speaker.Play(beep.Seq(vol, beep.Callback(func(){
		m.done <-true
	})))
	for {
		select {
		case <-m.done:
			return
		case m.paused =<-m.toggle:
			speaker.Lock()
			ctrl.Paused = m.paused
			speaker.Unlock()
		case volume :=<-m.volume:
			speaker.Lock()
			vol.Volume += volume
			speaker.Unlock()
		}
	}
}

func try_getsongs(name string, default_dir string) ([]string, error) {
	files := []string{}
	file, err := os.Stat(name)
	if err != nil { // NOTE: search the titlename within the default music dir
		// TODO: make this to get the matching title within the database
		fmt.Printf("Checking if title\n")
		dirfs := os.DirFS(default_dir)
		files, err = fs.Glob(dirfs, fmt.Sprintf("%s.ogg", name))
		// TODO: support other formats (mp3, wav, and etc..)
		if err != nil || files == nil {
			fmt.Printf("No file with name of %s found in %s!\n", name, default_dir)
			return files, err
		}
		return files, nil
	}
	fmt.Printf("Checking if dir or file: %s.\n", name)
	if !file.IsDir() {
		files = append(files, name)
		fmt.Printf("Found file %s\n", name)
	} else {
		dirpath := strings.TrimRight(name, "/")
		files, err = get_songs_from_dir(dirpath)
		if err != nil {
			return files, err
		}
	}
	return files, nil
}

// playlist should be additional args
func parse_args(config *Config) (cmd string, args []any) {
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
		list, err := try_getsongs(arg, config.MusicDir)
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

func get_songs_from_dir(dirpath string) ([]string, error) {
	songs := []string{}
	err := filepath.WalkDir(dirpath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirfs := os.DirFS(path)
			files, err := fs.Glob(dirfs, "*.ogg")
			if err == nil && len(files) > 0 {
				for _, song := range files {
					songs = append(songs, fmt.Sprintf("%s/%s", path, song))
				}
			}
		}
		return nil
	})
	if err != nil {
		return songs, err
	}
	return songs, nil
}
