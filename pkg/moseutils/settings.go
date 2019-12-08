// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"encoding/json"
	"log"
	"os"
)

// Settings represents the configuration information found in settings.json
type Settings struct {
	AttackOrgName     string
	ChefClientKey     string
	ChefNodeName      string
	ChefValidationKey string
	CleanupFile       string
	ContainerName     string
	PuppetBackupLoc   string
	RemoteHost        string
	SslCertPath       string
	SslKeyPath        string
	TargetChefServer  string
	TargetOrgName     string
	UploadFilePath    string
}

// LoadSettings will return the settings found in settings.json
func LoadSettings(jsonFile string) Settings {
	file, err := os.Open(jsonFile)
	if err != nil {
		log.Fatalf("Error loading settings: %s", err)
	}

	settings := Settings{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&settings); err != nil {
		log.Fatalf("Error decoding settings: %s", err)
	}
	return settings
}
