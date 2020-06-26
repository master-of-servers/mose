// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"bufio"
	"errors"
	"os"

	"github.com/rs/zerolog/log"
)

// RemoveTracker removes a file created with TrackChanges
func RemoveTracker(filePath string, osTarget string, destroy bool) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal().Msgf("Unable to open file %s, exiting...", filePath)
	}
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	var deleted map[string]bool
	var ans bool
	deleted = make(map[string]bool)
	for scanner.Scan() {
		// Ask to remove file for cleanup
		ans = false
		filename := scanner.Text()
		if deleted[filename] {
			continue
		}
		if !destroy {
			ans, err = AskUserQuestion("Would you like to remove this file: "+filename+"? ", osTarget)
			if err != nil {
				log.Fatal().Msg("Quitting cleanup...")
			}
		}
		if ans || destroy {
			err = os.RemoveAll(filename)
			if err != nil {
				ColorMsgf("Error removing file %s", filename)
			}
			deleted[filename] = true
		}
	}
}

// TrackChanges is used to track changes in a file
func TrackChanges(filePath string, content string) (bool, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, errors.New("failure with os.OpenFile")
	}
	defer f.Close()

	_, err = f.WriteString(content + "\n")
	if err != nil {
		return false, errors.New("failure writing the TrackChanges string")
	}

	return true, nil
}
