package main

import (
	"context"
	"github.com/l50/mose/pkg/utils"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Make sure no maliciousness can happen with overwriting a file on the operator's system
func checkInvalidChars(file string) {
	var disallowedChars = []string{
		"..",
		"~",
		"!",
		"@",
		"#",
		"$",
		"%",
		"^",
		"&",
		"*",
		"(",
		")",
		"+",
		"=",
		"{",
		"}",
		"]",
		"[",
		"|",
		"\\",
		"`",
		",",
		"/",
		"?",
		";",
		":",
		"'",
		"\"",
		"<",
		">"}

	for _, c := range disallowedChars {
		if strings.Contains(file, c) {
			log.Fatalf("Invalid character in the filename: %v", file)
		}
	}
}

func fileUpload(w http.ResponseWriter, r *http.Request) {
	log.Println("File Upload Endpoint Hit")

	r.ParseMultipartForm(10 << 20)

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	log.Printf("%v", handler.Header)

	checkInvalidChars(handler.Filename)

	f, err := os.OpenFile("keys/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	io.Copy(f, file)
	log.Printf("Successfully uploaded %v", handler.Filename)
}

func createUploadRoute() {
	// TODO: Use HTTPS - https://github.com/ryhanson/phishery/blob/master/phish/phishery.go
	ip, err := utils.GetLocalIP()
	if err != nil {
		log.Fatalln(err)
	}
	if _, err := os.Stat("keys"); os.IsNotExist(err) {
		utils.CreateFolders([]string{"keys"})
	}

	http.HandleFunc("/upload", fileUpload)
	msg("Listener being served at http://%s:%s/%s-%s for %d seconds", ip, "8081", cmTarget, osTarget, 10)
	srv := utils.StartHttpServer(8081, "")
	time.Sleep(10 * time.Second)

	info("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalln(err)
	}
}
