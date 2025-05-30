package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
)

var playlist = []string{} // current playlist

func main() {
	cmd := "start"
	path := parse_args(os.Args, &cmd)
	switch cmd {
	case "start":
		start(path)
	}
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
	file_path := fmt.Sprintf("./public/%s.ogg", name)
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
	args = slices.Delete(args, 1, 1)
	return arg, nil
}


func try_getpath(song string) (string, error) {
	_, err := os.Stat(song)
	if err == nil {
		return song, nil
	}
	// TODO: if the argument is a directory get all the music files and load to a temporary playlist to play.
	// TODO: if the argument is a title check all the files from the stored music paths and play the first one.
	return "", nil
}

func parse_args(args []string, cmd *string) string {
	args = slices.Delete(args, 0, 1)
	arg, err := extract_arg(args)
	// TODO: should be fetch from a persistent data where the value is the last song played
	// for now it is hardcoded to this song
	path := "./public/Lofi Girl - Snowman.ogg"
	if err != nil {
		return path
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
		path, err = try_getpath(arg)
		if err != nil {
			os.Exit(1)
		}
	case "toggle", "next", "prev", "stop", "list":
		*cmd = arg
	case "help":
		fmt.Print("TODO: add help\n")
		os.Exit(0)
	default:
		path, err = try_getpath(arg)
		if err != nil {
			msg := "%s is not a valid command or song\nUSAGE: apollo [COMMAND] [FILEPATH | DIRPATH | TITLE]\n"
			fmt.Fprintf(os.Stderr ,msg, arg)
			os.Exit(1)
		}
	}
	return path
}
