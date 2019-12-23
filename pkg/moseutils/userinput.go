// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"encoding/json"
	"github.com/fatih/color"
	"log"
	"path/filepath"
)

// UserInput holds all values from command line arguments and the settings.json
// This is a necessity resulting from templates needing to take values
// from a single struct, and MOSE taking user input from multiple sources
type UserInput struct {
	// CLI
	OSArch               string
	Cmd                  string
	Debug                bool
	ExfilPort            int
	FilePath             string
	FileUpload           string
	LocalIP              string
	PayloadName          string
	NoServe              bool
	OSTarget             string
	WebSrvPort           int
	RemoteUploadFilePath string
	Rhost                string
	SettingsPath         string
	ServeSSL             bool
	CMTarget             string
	TimeToServe          int

	// Settings
	ChefClientKey       string
	ChefNodeName        string
	ChefValidationKey   string
	CleanupFile         string
	ContainerName       string
	ImageName           string
	PuppetBackupLoc     string
	RemoteHost          string
	SSLCertPath         string
	SSLKeyPath          string
	TargetChefServer    string
	TargetOrgName       string
	TargetValidatorName string
	UploadFilePath      string
}

var (
	// User Input
	Cli          CliArgs
	JSONSettings Settings

	// Colorized output
	errmsg = color.Red
	info   = color.Yellow
	msg    = color.Green

	// CLI Parameters
	osArch               string
	cmd                  string
	debug                bool
	exfilPort            = 9090
	filePath             string
	fileUpload           string
	localIP              string
	payloadName          string
	osTarget             string
	webSrvPort           = 8090
	remoteUploadFilePath string
	rhost                string
	settingsPath         string
	serveSSL             bool
	cmTarget             string
	timeToServe          int
)

func processInput() {
	Cli = ParseCLIArgs()
	JSONSettings = loadSettings(Cli.SettingsPath)

	// If rhost isn't specified as an input parameter, set it to the value in settings.json
	if Cli.Rhost == "" {
		Cli.Rhost = JSONSettings.RemoteHost
	}
	if Cli.FileUpload != "" {
		Cli.FileUpload = filepath.Base(Cli.FileUpload)
	}
	if Cli.Debug {
		log.Print("JSON configuration loaded with the following values")
		b, err := json.MarshalIndent(JSONSettings, "", "  ")
		if err == nil {
			log.Println(string(b))
		}
	}
}

// GetUserInput returns all user input parameters specified through
// command line arguments and the settings.json file
func GetUserInput() UserInput {
	processInput()
	var UserInput = UserInput{
		ChefClientKey:        JSONSettings.ChefClientKey,
		ChefNodeName:         JSONSettings.ChefNodeName,
		ChefValidationKey:    JSONSettings.ChefValidationKey,
		CleanupFile:          JSONSettings.CleanupFile,
		Cmd:                  Cli.Cmd,
		CMTarget:             Cli.CMTarget,
		ContainerName:        JSONSettings.ContainerName,
		Debug:                Cli.Debug,
		ExfilPort:            Cli.ExfilPort,
		FilePath:             Cli.FilePath,
		FileUpload:           Cli.FileUpload,
		ImageName:            JSONSettings.ImageName,
		LocalIP:              Cli.LocalIP,
		OSArch:               Cli.OSArch,
		OSTarget:             Cli.OSTarget,
		PayloadName:          Cli.PayloadName,
		PuppetBackupLoc:      JSONSettings.PuppetBackupLoc,
		RemoteUploadFilePath: Cli.RemoteUploadFilePath,
		Rhost:                Cli.Rhost,
		SettingsPath:         Cli.SettingsPath,
		ServeSSL:             Cli.ServeSSL,
		SSLCertPath:          JSONSettings.SSLCertPath,
		SSLKeyPath:           JSONSettings.SSLKeyPath,
		TimeToServe:          Cli.TimeToServe,
		TargetChefServer:     JSONSettings.TargetChefServer,
		TargetOrgName:        JSONSettings.TargetOrgName,
		TargetValidatorName:  JSONSettings.TargetValidatorName,
		UploadFilePath:       JSONSettings.UploadFilePath,
		WebSrvPort:           Cli.WebSrvPort,
	}
	return UserInput
}
