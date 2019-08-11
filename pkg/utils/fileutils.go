package utils

import (
	//	"fmt"
	"bufio"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
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
		// DEBUG
		//log.Printf("%s exists", fileLoc)
		return true
	}
	// DEBUG
	//log.Printf("%s does not exist", fileLoc)
	return false
}

// Based on: https://gist.github.com/fubarhouse/5ae3fdd5ed5be9e718a92d9b3c780a22
// with support for a number of directories to enable more strategic searches
func GetFileList(searchDirs []string) []string {
	fileList := []string{}
	for _, dir := range searchDirs {
		filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
			fileList = append(fileList, path)
			return nil
		})
	}
	return fileList
}

// https://siongui.github.io/2017/01/30/go-insert-line-or-string-to-file/
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
