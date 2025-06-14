package main

import (
	"database/sql"
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

type Playlist struct {
	id int
	name string
	songs []Music
}

type Music struct {
	id int
	title string
	path string
}

type MusicManager struct {
	playlist Playlist
	playing bool
	current int
	paused bool
	config *Config
	db *sql.DB

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
	cmd, args := parse_cmds()

	config := get_config()
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

	db, err := get_db()
	if err != nil {
		fmt.Printf("Error getting db: %v\n", err)
		return
	}
	defer db.Close()
	playlist := Playlist{
		id: 0,
		name: "Unlisted",
		songs: []Music{},
	}

	if args != nil || len(args) > 0 {
		for _, v := range args {
			playlist.songs = append(playlist.songs, v.(Music))
		}
	}

	if playlist.length() == 0 {
		playlist.songs = get_all_songs(db)
		playlist.name = "All Songs"
	}

	manager := MusicManager{
		playlist: playlist,
		config: config,
		current: 0,
		playing: false,
		toggle: make(chan bool),
		volume: make(chan float64),
		done: make(chan bool),
		db: db,
	}

	start_rpc(&dmon, &manager)
	// start_musicplayer()
	// start_http()
}

func (p *Playlist) length() int {
	return len(p.songs)
}

func (p *Playlist) add_songs(songs []Music) {
	for _, song := range songs {
		p.songs = append(p.songs, song)
	}
}

func (m *MusicManager) current_song() *Music {
	return &m.playlist.songs[m.current]
}

func (d *Daemon) Kill(args string, reply *string) error {
	*reply = "Daemon Killed"
	d.context.Release()
	save_config(d.config)
	os.Exit(0)
	return nil
}

func (m *MusicManager) Play(args string, reply *string) error {
	if args != "" && args != m.playlist.name {
		playlist, err := get_playlist(m.db, args)
		if err != nil {
			fmt.Printf("Error: getting playlist from db: %v\n", err)
			return fmt.Errorf("Error: getting playlist from db: %v\n", err)
		}
		// stops current playlist
		if m.playing {
			m.playing = false
			m.toggle <-true
			m.done <-true
		}
		// resets playlist index and sets the new playlist
		*reply = fmt.Sprintf("Switching playlist to '%s'\n", playlist.name)
		m.current = 0
		m.playlist = playlist
	}
	if !m.playing {
		if m.playlist.length() == 0 {
			*reply = fmt.Sprintf("%sCan't play '%s', has 0 songs", *reply, m.playlist.name)
			return nil
		}
		*reply = *reply + "Playing Song: " + m.current_song().title
		go m.play_playlist()
	} else {
		if m.paused == true {
			m.toggle <-false
			*reply = fmt.Sprintf("Unpausing '%s'", m.playlist.name)
		}
		*reply = "Already Playing song...."
	}
	return nil
}

func (m *MusicManager) Clean(args string, reply *string) error {
	changes := clean_musics(m.db)
	*reply = fmt.Sprintf("Cleaned %d item(s) in the database", changes)
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
	if m.playlist.length() == 0 {
		return errors.New("No songs in playlist")
	}
	if m.current == 0 {
		m.current = m.playlist.length() - 1
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

func (m *MusicManager) Playlist(args string, reply *string) error {
	if m.playlist.length() == 0 {
		*reply = fmt.Sprintf("Playlist: [%d] %s\nNo songs", m.playlist.id ,m.playlist.name)
	} else {
		*reply = fmt.Sprintf("Playlist: [%d] %s\n", m.playlist.id, m.playlist.name)
		for i, song := range m.playlist.songs {
			title := song.title
			if i == m.current {
				title = title + " <- [Selected]"
			}
			row := fmt.Sprintf("%d. %s\n", i+1, title)
			*reply = *reply + row
		}
	}
	return nil
}

func (m *MusicManager) Next(args string, reply *string) error {
	if m.playlist.length() == 0 {
		return errors.New("No songs in playlist")
	}
	if m.playing {
		m.toggle <-true
		m.done <-true
		m.toggle <-false
		*reply = "Going next"
	} else{
		m.current++
		if m.playlist.length() == m.current {
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

func (m *MusicManager) List(args string, reply *string) error {
	*reply = list_musics(m.db)
	return nil
}

func (m *MusicManager) Sync(args string, reply *string) error {
	var err error
	*reply, err = sync_musics(m.db, args, m.config.MusicDir)
	return err
}

func (m *MusicManager) Create(args string, reply *string) error {
	var err error
	*reply, err = create_playlist(m.db, args)
	return err
}

func (m *MusicManager) Delete(args string, reply *string) error {
	var err error
	*reply, err = delete_playlist(m.db, args)
	return err
}

func (m *MusicManager) Playlists(args string, reply *string) error {
	var err error
	*reply, err = list_playlist(m.db)
	return err
}

func (m *MusicManager) Add(args []int, reply *string) error {
	var err error
	songs, err := add_songs(m.db ,m.playlist.id, args)
	if err != nil {
		*reply = fmt.Sprintf("Error: adding songs to playlist:%v", err)
		return fmt.Errorf("Error: adding songs to playlist:%v", err)
	}
	m.playlist.add_songs(songs)
	*reply = fmt.Sprintf("Added %d songs to '%s' playlist", len(songs), m.playlist.name)
	return err
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
	if m.playlist.length() == 0 {
		fmt.Printf("No songs in playlist\n")
		return
	}
	m.paused = false
	m.playing = true
	fmt.Printf("Playlist Playing...!\n")
	for m.playing && m.current < m.playlist.length() {
		file_path := m.current_song().path

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
			if m.config.Loop && m.playlist.length() == m.current {
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

func try_getsongs(name string) ([]Music, error) {
	songs := []Music{}
	file, err := os.Stat(name)
	if err != nil {
		db, err := get_db()
		if err != nil {
			fmt.Printf("Cannot open database in in args..\n")
		} else {
			song, err := get_song(db, name)
			if err != nil {
				fmt.Printf("Cannot get song %s in database\n", name)
			}
			songs = append(songs, song)
		}
		defer db.Close()
		return songs, nil
	}
	fmt.Printf("Checking if dir or file: %s.\n", name)
	if !file.IsDir() {
		title := strings.TrimSuffix(file.Name(), ".ogg")
		songs = append(songs, Music{0, title, name})
		fmt.Printf("Found file %s\n", name)
	} else {
		dirpath := strings.TrimRight(name, "/")
		songs, err = get_songs_from_dir(dirpath)
		if err != nil {
			return songs, err
		}
	}
	return songs, nil
}

func has_args() bool {
	return len(os.Args) > 2
}

func parse_cmds() (cmd string, args []any) {
	if len(os.Args) == 1 {
		return "start", args
	}
	arg := os.Args[1]
	switch arg {
	case "playlist", "toggle", "next", "prev", "stop", "list", "kill", "clean", "playlists":
		return arg, args
	// TODO: Implement case
	case "add":
		cmd := arg
		if !has_args() {
			fmt.Fprintf(os.Stderr, "ERROR: missing argument to add \n")
			fmt.Fprintf(os.Stderr, "USAGE: apollo add [SONG IDS...]\n")
			os.Exit(1)
		}
		args := []any{}
		for _,v := range os.Args[2:] {
			args = append(args, v)
		}
		return cmd, args
	case "play":
		cmd := arg
		if has_args() {
			arg = os.Args[2]
			args = make([]any, 1)
			args[0] = arg
		}
		return cmd, args
	case "create", "delete":
		cmd := arg
		if !has_args() {
			fmt.Fprintf(os.Stderr, "ERROR: missing argument to %s\n", cmd)
			fmt.Fprintf(os.Stderr, "USAGE: apollo %s [PLAYLIST NAME]\n", cmd)
			os.Exit(1)
		}
		arg = os.Args[2]
		args = make([]any, 1)
		args[0] = arg
		return cmd, args
	case "sync":
		cmd = arg
		if has_args() {
			arg := os.Args[2]
			info, err := os.Stat(arg)
			if err != nil || !info.IsDir() {
				fmt.Fprintf(os.Stderr, "ERROR: invalid argument to sync '%s'\n", arg)
				fmt.Fprintf(os.Stderr, "USAGE: apollo sync [DIRPATH]\n")
				os.Exit(1)
			}
			args = make([]any, 1)
			args[0] = arg
		}
		return cmd, args
	case "vol":
		if !has_args() {
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
		songs, err := try_getsongs(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr ,"ERROR: %s is not a valid song argument or command\n", arg)
			fmt.Fprintf(os.Stderr ,"USAGE: apollo [COMMAND | FILEPATH | DIRPATH | TITLE] \n")
			os.Exit(1)
		}
		args = make([]any, len(songs))
		for i, v := range songs {
			args[i] = v
		}
	}
	return "start", args
}

func get_songs_from_dir(dirpath string) ([]Music, error) {
	songs := []Music{}
	err := filepath.WalkDir(dirpath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirfs := os.DirFS(path)
			files, err := fs.Glob(dirfs, "*.ogg")
			if err == nil && len(files) > 0 {
				for _, song := range files {
					path := fmt.Sprintf("%s/%s", path, song)
					songs = append(songs, Music{0, song, path})
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
