// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software

package main

import (
	"os"
	"testing"

	utils "github.com/l50/goutils"
)

func TestBackupManifest(t *testing.T) {
	file := "site.pp"
	backupFile := "site.pp.bak.mose"
	created := utils.CreateEmptyFile(file)
	BackupManifest(file)
	if !created || !utils.FileExists(backupFile) {
		t.Fatalf("Failed to create backup of the %s manifest.", file)
	} else {
		os.Remove(file)
		os.Remove(backupFile)
	}
}
