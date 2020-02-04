// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

// CpFile is used to copy a file from a source (src) to a destination (dst)
// If there is a failure to do so, an error is returned
func CpFile(src string, dst string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		log.Printf("Error reading from %s: %v", src, err)
		return err
	}

	err = ioutil.WriteFile(dst, input, 0644)
	if err != nil {
		log.Printf("Error writing to %s: %v", dst, err)
		return err
	}
	return nil
}

// Cd changes the directory to the one specified with dir
func Cd(dir string) {
	err := os.Chdir(dir)
	if err != nil {
		log.Fatalln(err)
	}
}

// FindFiles finds files based on their extension in specified directories
// locations - where to search for files
// extensionList - file extensions to search for
// fileNames - filenames to search for
// dirNames - directory names to search for (optional)
// Returns files found that meet the input criteria
func FindFiles(locations []string, extensionList []string, fileNames []string, dirNames []string) ([]string, []string) {
	var foundFiles = make(map[string]int)
	var foundDirs = make(map[string]int)
	fileList, dirList := GetFileAndDirList(locations)
	//  iterate through filenames if they are provided
	for _, fileContains := range fileNames {
		for _, file := range fileList {
			if strings.Contains(file, fileContains) {
				if _, exist := foundFiles[file]; !exist {
					foundFiles[file] = 1
				}
			}
		}
	}
	// iterate through the extensionList (if it's provided)s
	for _, ext := range extensionList {
		for _, file := range fileList {
			if strings.HasSuffix(file, ext) {
				if _, exist := foundFiles[file]; !exist {
					foundFiles[file] = 1
				}
			}
		}
	}
	// iterate through dirNames (if they're provided)
	for _, reg := range dirNames {
		for _, dir := range dirList {
			m, err := regexp.MatchString(reg, dir)
			if err != nil {
				log.Fatalf("Unable to locate the %s directory: %v\n", dir, err)
			} else {
				if m {
					if _, exist := foundDirs[dir]; !exist {
						foundDirs[dir] = 1
					}
				}
			}
		}
	}

	if len(foundDirs) == 0 && len(dirNames) > 0 {
		log.Printf("No dirs found with these names: %v", dirNames)
	}
	if len(foundFiles) == 0 && len(fileNames) > 0 {
		log.Printf("Unable to find any files with these names: %v", fileNames)
	}

	foundFileKeys := make([]string, 0, len(foundFiles))
	foundDirsKeys := make([]string, 0, len(foundDirs))
	for k := range foundFiles {
		foundFileKeys = append(foundFileKeys, k)
	}
	for k := range foundDirs {
		foundDirsKeys = append(foundDirsKeys, k)
	}

	return foundFileKeys, foundDirsKeys
}

// FindFile locates a file (fileName) in a list of input directories (dir)
// If the file is found, then it returns true along with the file location
// Otherwise it returns false with an empty string
func FindFile(fileName string, dirs []string) (bool, string) {
	fileList, _ := GetFileAndDirList(dirs)
	for _, file := range fileList {
		fileReg := `/` + fileName + `$`
		m, err := regexp.MatchString(fileReg, file)
		if err != nil {
			log.Fatalf("We had an issue locating the %v file: %v\n", fileReg, err)
		} else {
			if m {
				return true, file
			}
		}
	}
	return false, ""
}

// CreateFilePath will create a file specified
// Check prefixes of path that normal filepath package won't expand inherantly
// if it matches any prefix $HOME, ~/, / then we need to treat them seperately
func CreateFilePath(text string, baseDir string) (string, error) {
	var path string
	_, err := user.Current()
	if err != nil {
		return "", err
	}
	if filepath.HasPrefix(text, "~/") || filepath.HasPrefix(text, "$HOME") {
		path = filepath.Base(text)
		_, path = FindFile(path, []string{"/root", "/home"})
	} else if filepath.HasPrefix(text, "/") {
		path = text
	} else {
		path, err = filepath.Abs(filepath.Join(baseDir, text))
		if err != nil {
			return "", err
		}
	}

	return path, nil
}
