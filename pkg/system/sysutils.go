// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package system

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
)

// CpFile is used to copy a file from a source (src) to a destination (dst)
// If there is a failure to do so, an error is returned
func CpFile(src string, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		log.Error().Err(err).Msgf("Error reading from %s", src)
		return err
	}

	err = os.WriteFile(dst, input, 0644)
	if err != nil {
		log.Error().Err(err).Msgf("Error writing to %s", dst)
		return err
	}
	return nil
}

// Cd changes the directory to the one specified with dir
func Cd(dir string) {
	err := os.Chdir(dir)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

// GetUIDGid gets the uid and gid of a file
func GetUIDGid(file string) (int, int, error) {
	info, err := os.Stat(file)
	if err != nil {
		return -1, -1, err
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		UID := int(stat.Uid)
		GID := int(stat.Gid)
		return UID, GID, nil
	}
	return -1, -1, errors.New("unable to retrieve UID and GID of file")
}

// ChownR recursively change owner of directory
func ChownR(path string, uid int, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(name, uid, gid)
	})
}

// CreateFilePath will create a file specified
// Check prefixes of path that normal filepath package won't expand inherently
// if it matches any prefix $HOME, ~/, / then we need to treat them separately
func CreateFilePath(text string, baseDir string) (string, error) {
	var path string
	switch {
	case strings.HasPrefix(text, "~/") || strings.HasPrefix(text, "$HOME"):
		path = filepath.Base(text)
		_, path = FindFile(path, []string{"/root", "/home"})
	case strings.HasPrefix(text, "/"):
		path = text
	default:
		var err error
		path, err = filepath.Abs(filepath.Join(baseDir, text))
		if err != nil {
			return "", err
		}
	}

	return path, nil
}

// RunCommand runs a specified system command
func RunCommand(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("run %s %v: %w (output: %s)", cmd, args, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// CheckRoot will check to see if the process is being run as root
func CheckRoot() {
	if os.Geteuid() != 0 {
		log.Fatal().Msg("This script must be run as root.")
	}
}

// Gwd will return the current working directory
func Gwd() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	return dir
}
