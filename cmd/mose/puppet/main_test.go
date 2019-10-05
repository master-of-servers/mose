package main

// Copyright 2019 Jayson Grace. All rights reserved
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

import (
	"github.com/l50/goutils"
	"os"
	"testing"
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