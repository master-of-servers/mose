// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

func CpFile(src string, dst string) {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		log.Println(err)
		return
	}

	err = ioutil.WriteFile(dst, input, 0644)
	if err != nil {
		log.Printf("Error creating %v: %v", dst, err)
		return
	}
}

func Cd(dir string) {
	err := os.Chdir(dir)
	if err != nil {
		log.Fatalln(err)
	}
}

// FindFiles finds based on their file extension in specified directories
// locations: slice with locations to search for files
// extensionList: slice with file extensions to check for
// fileNames: slice with filenames to search for
// Returns files found that meet the input criteria
func FindFiles(locations []string, extensionList []string, fileNames []string, dirNames []string) ([]string, []string) {
	var foundFiles = make(map[string]int)
	var foundDirs = make(map[string]int)
	fileList, dirList := GetFileAndDirList(locations)
	// if filenames are supplied, then iterate through them
	for _, fileContains := range fileNames {
		for _, file := range fileList {
			if strings.Contains(file, fileContains) {
				if _, exist := foundFiles[file]; !exist {
					foundFiles[file] = 1
				}
			}
		}
	}
	// if extensionList is supplied iterated through them
	for _, ext := range extensionList {
		for _, file := range fileList {
			if strings.HasSuffix(file, ext) {
				if _, exist := foundFiles[file]; !exist {
					foundFiles[file] = 1
				}
			}
		}
	}
	// If dirnames are supplied iterate through them
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
		log.Printf("No dirs found with names %#v", dirNames)
	}
	if len(foundFiles) == 0 && len(fileNames) > 0 {
		log.Printf("Unable to find any files with names %#v", fileNames)
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

func FindBin(binName string, dirs []string) (bool, string) {
	fileList, _ := GetFileAndDirList(dirs)
	for _, file := range fileList {
		fileReg := `\b` + binName + `$\b`
		m, err := regexp.MatchString(fileReg, file)
		if err != nil {
			log.Fatalf("We had an issue locating the %v binary: %v\n", fileReg, err)
		} else {
			if m {
				return true, file
			}
		}
	}
	return false, ""
}
