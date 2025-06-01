package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
)

var music_dir = "./public"
var current = 0
var loop = true

func main() {
	cmd := "start"
	playlist := parse_args(os.Args, &cmd)
	switch cmd {
	case "play":
		play(playlist)
	// case "start":
	}
}

func play(playlist []string) {
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
}

func play_song(s beep.StreamSeekCloser, format beep.Format) {
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	done := make(chan bool)
	speaker.Play(beep.Seq(s, beep.Callback(func(){
		done <- true
	})))
	<- done
}

func start(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Failed to open ogg file %s\n", path)
		os.Exit(2)
	}
	defer file.Close()

	streamer, format, err := vorbis.Decode(file)
	if err != nil {
		fmt.Printf("Failed to decode %s\n", path)
		os.Exit(2)
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	fmt.Printf("Playing: %s\n", path)

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func(){
		done <- true
	})))
	<- done
}

func musicHandler(w http.ResponseWriter, r *http.Request) {
    name := strings.TrimPrefix(r.URL.Path, "/musics/")
	file_path := fmt.Sprintf("%s/%s.ogg", music_dir, name)
	_, err := os.Stat(file_path)
	if err != nil {
		http.Error(w, fmt.Sprintf("File:'%s' does not Exist -> %s\n", name, r.URL.Path), 500)
		return
	}
	fmt.Printf("Playing %s\n", file_path)
	http.ServeFile(w, r, file_path)
}

func extract_arg(args []string) (string, error){
	if len(args) == 0 {
		return "", errors.New("No arguments specified")
	}
	arg := args[0]
	args = slices.Delete(args, 0, 1)
	return arg, nil
}


func try_getpath(name string) ([]string, error) {
	files := []string{}
	file, err := os.Stat(name)
	if err == nil && !file.IsDir() {
		files = append(files, name)
		fmt.Printf("Found file %s\n", name)
		return files, nil
	} else if file.IsDir() {
		dirfs := os.DirFS(strings.TrimRight(name, "/"))
		// TODO: support other formats (mp3, wav, and etc..)
		songs, err := fs.Glob(dirfs, "*.ogg")
		if err == nil && len(songs) > 0 {
			for _, song := range songs {
				files = append(files, fmt.Sprintf("%s/%s", name, song))
			}
			return files, nil
		}
	}
	// TODO: make this option use the database the get the name of the file and
	// get its loc where the location is the primary key
	dirfs := os.DirFS(music_dir)
	files, err = fs.Glob(dirfs, fmt.Sprintf("%s.ogg", name))
	// TODO: support other formats (mp3, wav, and etc..)
	if err != nil || files == nil {
		fmt.Printf("No file with name of %s found!\n", name)
		return files, errors.New("File with name of %s does not exist!\n")
	}
	return files, nil
}

func parse_args(args []string, cmd *string) []string {
	args = slices.Delete(args, 0, 1)
	arg, err := extract_arg(args)
	// TODO: should be fetch from a persistent data where the value is the last
	// song played for now it is hardcoded to this song
	files := []string{"./public/Lofi Girl - Snowman.ogg"}
	if err != nil {
		return files
	}
	switch arg {
	case "play":
		*cmd = arg
		arg, err = extract_arg(args)
		if err != nil {
			msg := "No music to play\nUSAGE: apollo play [FILEPATH | DIRPATH | TITLE]\n"
			fmt.Fprint(os.Stderr ,msg)
			os.Exit(1)
		}
		files, err = try_getpath(arg)
		if err != nil {
			os.Exit(1)
		}
	// TODO: remove this if can make the program like act like daemon.
	case "toggle", "next", "prev", "stop", "list":
		*cmd = arg
	case "help":
		fmt.Print("TODO: add help\n")
		os.Exit(0)
	default:
		files, err = try_getpath(arg)
		if err != nil {
			msg := "ERROR: %s is not a valid song or command\nUSAGE: apollo [COMMAND] [FILEPATH | DIRPATH | TITLE] or apollo [FILEPATH | DIRPATH | TITLE]\n"
			fmt.Fprintf(os.Stderr ,msg, arg)
			os.Exit(1)
		}
	}
	return files
}
