// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"bufio"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
)

// RemoveTracker removes a file created with TrackChanges
func RemoveTracker(filePath string, osTarget string, destroy bool) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal().Err(err).Msgf("Unable to open file %s, exiting...", filePath)
	}

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	deleted := make(map[string]struct{})
	for scanner.Scan() {
		// Ask to remove file for cleanup
		filename := scanner.Text()
		if _, seen := deleted[filename]; seen {
			continue
		}
		answer := false
		if !destroy {
			answer, err = AskUserQuestion("Would you like to remove this file: "+filename+"? ", osTarget)
			if err != nil {
				_ = f.Close()
				log.Fatal().Err(err).Msg("Quitting cleanup...")
			}
		}
		if answer || destroy {
			if err = os.RemoveAll(filename); err != nil {
				ColorMsgf("Error removing file %s", filename)
			}
			deleted[filename] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		_ = f.Close()
		log.Fatal().Err(err).Msg("Failed while reading tracker file")
	}
	if err := f.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close tracker file")
	}
}

// TrackChanges is used to track changes in a file
func TrackChanges(filePath string, content string) (bool, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("open track file %s: %w", filePath, err)
	}
	defer f.Close()

	if _, err = f.WriteString(content + "\n"); err != nil {
		return false, fmt.Errorf("write track entry: %w", err)
	}

	return true, nil
}
