// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"encoding/json"
	"log"
	"os"
)

// settings represents the configuration information found in settings.json
type Settings struct {
	AttackOrgName     string
	ChefClientKey     string
	ChefNodeName      string
	ChefValidationKey string
	CleanupFile       string
	ContainerName     string
	ImageName		  string
	PuppetBackupLoc   string
	RemoteHost        string
	SSLCertPath       string
	SSLKeyPath        string
	TargetChefServer  string
	TargetOrgName     string
	UploadFilePath    string
}

// loadSettings will return the settings found in settings.json
func loadSettings(jsonFile string) Settings {
	file, err := os.Open(jsonFile)
	if err != nil {
		log.Fatalf("Error loading settings: %s", err)
	}

	jsonSettings := Settings{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&jsonSettings); err != nil {
		log.Fatalf("Error decoding settings: %s", err)
	}

	return jsonSettings
}
