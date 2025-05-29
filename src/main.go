package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// TODO: play a ogg file locally on a separate thread in the server machine and give detailes of the current one playing in the root.
// - [ ] create music file parser that can play music.

// TODO: details to give in root:
// - title
// - current time
// - next and prev

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

func main() {
	socket := ":3000"
	http.HandleFunc("/musics/", musicHandler)
	fmt.Printf("Starting server at: http://localhost%s\n", socket);
	http.ListenAndServe(socket, nil)
}
