// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"encoding/json"
	"log"
	"os"
)

type Settings struct {
	SslCertPath       string
	SslKeyPath        string
	ChefNodeName      string
	ChefClientKey     string
	ChefValidationKey string
	AttackOrgName     string
	TargetOrgName     string
	TargetIP          string
	UploadFilePath    string
	CleanupFile       string
	PuppetBackupLoc   string
	RemoteHosts       string
	ContainerName     string
}

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
