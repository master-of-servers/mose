// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mholt/archiver"
)

func CreateFolders(folders []string) bool {
	for _, f := range folders {
		err := os.MkdirAll(f, os.ModePerm)
		if err != nil {
			log.Println(err)
			return false
		}
		log.Printf("Creating folder %s", f)
	}
	return true
}

func FileExists(fileLoc string) bool {
	if _, err := os.Stat(fileLoc); err == nil {
		return true
	}
	return false
}

func GrepFile(filePath string, regex *regexp.Regexp) []string {
	file, err := ioutil.ReadFile(filePath)

	if err != nil {
		log.Printf("Unable to read file %v", filePath)
	}
	contents := string(file)
	matches := regex.FindAllString(contents, -1)
	return matches
}

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

func File2lines(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LinesFromReader(f)
}

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

func TarFiles(files []string, tarLocation string) {
	tar := archiver.Tar{
		OverwriteExisting: true,
	}

	if err := tar.Archive(files, tarLocation); err != nil {
		log.Fatalln(err)
	}
}

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
				log.Fatal("Quitting cleanup ...")
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
