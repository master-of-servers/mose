// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"flag"
	"os"
)

// CliArgs holds command line arguments specified through user input
type CliArgs struct {
	OSArch               string
	Cmd                  string
	Debug                bool
	ExfilPort            int
	FilePath             string
	FileUpload           string
	LocalIP              string
	PayloadName          string
	OSTarget             string
	WebSrvPort           int
	RemoteUploadFilePath string
	SettingsPath         string
	ServeSSL             bool
	CMTarget             string
	TimeToServe          int
	Rhost                string
}

func setFlags() {
	flag.StringVar(&osArch, "a", "amd64", "Architecture that the target CM tool is running on")
	flag.StringVar(&cmd, "c", "", "Command to run on the targets")
	flag.BoolVar(&debug, "d", false, "Display debug output")
	flag.IntVar(&exfilPort, "ep", 443, "Port used to exfil data from chef server (default 443 with ssl, 9090 without)")
	flag.StringVar(&filePath, "f", "", "Output binary locally at <filepath>")
	flag.StringVar(&fileUpload, "fu", "", "File upload option")
	flag.StringVar(&localIP, "l", "", "Local IP Address")
	flag.StringVar(&payloadName, "m", "my_cmd", "Name for backdoor payload")
	flag.StringVar(&osTarget, "o", "linux", "Operating system that the target CM tool is on")
	flag.IntVar(&webSrvPort, "p", 443, "Port used to serve payloads on (default 443 with ssl, 8090 without)")
	flag.StringVar(&remoteUploadFilePath, "rfp", "/root/.definitelynotevil", "Remote file path to upload a script to (used in conjunction with -fu)")
	flag.StringVar(&rhost, "r", "", "Set the remote host for /etc/hosts in the chef workstation container (format is hostname:ip)")
	flag.StringVar(&settingsPath, "s", "settings.json", "JSON file to load for MOSE")
	flag.BoolVar(&serveSSL, "ssl", false, "Serve payload over TLS")
	flag.StringVar(&cmTarget, "t", "puppet", "Configuration management tool to target")
	flag.IntVar(&timeToServe, "tts", 60, "Number of seconds to serve the payload")
	flag.Parse()
}

// validateInput ensures that the user inputs proper arguments into mose.
func validateInput() bool {
	if cmd == "" && fileUpload == "" {
		ErrMsg("You must specify a cm target, a command, and an operating system.")
		ErrMsg("Example: mose -t puppet -c pwd -o Linux")
		return false
	}
	return true
}

// usage prints the usage instructions for MOSE.
func usage() {
	os.Args[0] = os.Args[0] + " [options]"
	flag.Usage()
	os.Exit(1)
}

func ParseCLIArgs() CliArgs {
	setFlags()
	CliArgs := CliArgs{
		OSArch:               osArch,
		Cmd:                  cmd,
		Debug:                debug,
		ExfilPort:            exfilPort,
		FilePath:             filePath,
		FileUpload:           fileUpload,
		LocalIP:              localIP,
		PayloadName:          payloadName,
		OSTarget:             osTarget,
		WebSrvPort:           webSrvPort,
		RemoteUploadFilePath: remoteUploadFilePath,
		SettingsPath:         settingsPath,
		ServeSSL:             serveSSL,
		CMTarget:             cmTarget,
		TimeToServe:          timeToServe,
		Rhost:                rhost,
	}

	if flag.NFlag() == 0 || !validateInput() {
		usage()
	}

	return CliArgs
}
