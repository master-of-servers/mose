// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mholt/archiver"
)

// CreateFolders creates folders specified in an input slice (folders)
// It returns true if it is able to create all of the folders
// Otherwise it returns false
func CreateFolders(folders []string) bool {
	for _, f := range folders {
		err := os.MkdirAll(f, os.ModePerm)
		if err != nil {
			log.Println(err)
			return false
		}
		fmt.Printf("Creating folder %s\n", f)
	}
	return true
}

// FileExists returns true if a file input (fileLoc) exists on the filesystem
// Otherwise it returns false
func FileExists(fileLoc string) bool {
	if _, err := os.Stat(fileLoc); err == nil {
		return true
	}
	return false
}

// GrepFile looks for patterns in a file (filePath) using an input regex (regex)
// It will return any matches that are found in the file in a slice
func GrepFile(filePath string, regex *regexp.Regexp) []string {
	file, err := ioutil.ReadFile(filePath)

	if err != nil {
		log.Printf("Unable to read file %v", filePath)
	}
	contents := string(file)
	matches := regex.FindAllString(contents, -1)
	return matches
}

// GetFileAndDirList gets all of the files and directories specified the initial search location (searchDirs)
// It returns a list of files and a list of directories found
func GetFileAndDirList(searchDirs []string) ([]string, []string) {
	fileList := []string{}
	dirList := []string{}
	for _, dir := range searchDirs {
		e := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
			if err == nil {
				if f.IsDir() {
					dirList = append(dirList, path)
				} else {
					fileList = append(fileList, path)
				}
			}
			return nil
		})
		if e != nil {
			log.Fatalln(e)
		}
	}
	return fileList, dirList
}

// File2lines returns a slice that contains all of the lines of the file specified with filePath
// Resource: https://siongui.github.io/2017/01/30/go-insert-line-or-string-to-file/
func File2lines(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LinesFromReader(f)
}

// ReadBytesFromFile returns all data from the input file (filePath) as a byte array
func ReadBytesFromFile(filePath string) ([]byte, error) {
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// WriteBytesToFile writes a byte array to the file specified with filePath with the permissions specified in perm
// An error will be returned if there is one
func WriteBytesToFile(filePath string, data []byte, perm os.FileMode) error {
	err := ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// InsertStringToFile with insert a string (str) into the n-th line (index) of a specified file (path)
// Resource: https://siongui.github.io/2017/01/30/go-insert-line-or-string-to-file/
func InsertStringToFile(path, str string, index int) error {
	lines, err := File2lines(path)
	if err != nil {
		return err
	}

	fileContent := ""
	for i, line := range lines {
		if i == index {
			fileContent += str
		}
		fileContent += line
		fileContent += "\n"
	}

	return ioutil.WriteFile(path, []byte(fileContent), 0644)
}

// LinesFromReader will return the lines read from a reader
// Resource: https://siongui.github.io/2017/01/30/go-insert-line-or-string-to-file/
func LinesFromReader(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// TarFiles will create a tar file at a specific location (tarLocation) with the files specified (files)
func TarFiles(files []string, tarLocation string) {
	tar := archiver.Tar{
		OverwriteExisting: true,
	}

	if err := tar.Archive(files, tarLocation); err != nil {
		log.Fatalln(err)
	}
}

// ReplLineInFile will replace a line in a file (filePath) with the specified replStr and delimiter (delim)
// It will return true with the path to the file if successful
// Otherwise it will return false and an empty string
func ReplLineInFile(filePath string, delim string, replStr string) (bool, string) {
	input, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalln(err)
		return false, ""
	}

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, delim) {
			lines[i] = replStr
		}
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(filePath, []byte(output), 0644)
	if err != nil {
		log.Fatalln(err)
		return false, ""
	}
	return true, filePath
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

// RemoveTracker removes a file created with TrackChanges
func RemoveTracker(filePath string, osTarget string, destroy bool) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Unable to open file %s, exiting...", filePath)
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
			ans, err = AskUserQuestion("Would you like to remove this file/folder "+filename, osTarget)
			if err != nil {
				log.Fatal("Quitting cleanup...")
			}
		}
		if ans || destroy {
			err = os.RemoveAll(filename)
			if err != nil {
				log.Printf("Error removing file %s", filename)
			}
			deleted[filename] = true
		}
	}
}
