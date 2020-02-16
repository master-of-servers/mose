// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software
package moseutils

import (
	"os"
	"testing"

	utils "github.com/l50/goutils"
)

func TestCpFile(t *testing.T) {
	ogFile := "test.txt"
	newFile := "copiedTest.txt"
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(ogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("appended some data\n")); err != nil {
		f.Close() // ignore error; Write error takes precedence
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	CpFile(ogFile, newFile)
	exists := utils.FileExists(newFile)
	if !exists {
		t.Fatal("Copy functionality is not working!")
	} else {
		os.Remove(ogFile)
		os.Remove(newFile)
	}
}
