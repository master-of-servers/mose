// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package system

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/mholt/archiver"
	"github.com/rs/zerolog/log"
)

// CreateFolders creates folders specified in an input slice (folders)
// It returns true if it is able to create all of the folders
// Otherwise it returns false
func CreateFolders(folders []string) bool {
	for _, f := range folders {
		err := os.MkdirAll(f, os.ModePerm)
		if err != nil {
			log.Error().Err(err).Msg("")
			return false
		}
		moseutils.ColorMsgf("Creating folder %s", f)
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
		log.Error().Msgf("Unable to read file %v", filePath)
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
			log.Fatal().Err(e).Msg("Error getting files and directories")
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

// ArchiveFiles will create an archive file at a specific location (archiveLocation) with the files specified (files)
// currently only supports tar and zip based archives. Rar can handle unpacking only and gz does not handle files
func ArchiveFiles(files []string, archiveLocation string) (string, error) {
	ext, err := archiver.ByExtension(filepath.Base(archiveLocation))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(archiveLocation); !os.IsNotExist(err) {
		_ = os.Remove(archiveLocation)
	}

	arc, ok := ext.(archiver.Archiver)
	if !ok {
		return "", errors.New("Archive type not supported currently currently supported: (tar.gz, tar, tar.xz, zip)")
	}
	if err := arc.Archive(files, archiveLocation); err != nil {
		return "", err
	}
	return archiveLocation, nil
}

// ReplLineInFile will replace a line in a file (filePath) with the specified replStr and delimiter (delim)
// It will return true with the path to the file if successful
// Otherwise it will return false and an empty string
func ReplLineInFile(filePath string, delim string, replStr string) (bool, string) {
	input, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatal().Err(err).Msg("")
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
		log.Fatal().Err(err).Msg("")
		return false, ""
	}
	return true, filePath
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
	// iterate through filenames if they are provided
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
				log.Fatal().Err(err).Msgf("Unable to locate the %s directory", dir)
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
		log.Info().Msgf("No dirs found with these names: %v", dirNames)
	}
	if len(foundFiles) == 0 && len(fileNames) > 0 {
		log.Info().Msgf("Unable to find any files with these names: %v", fileNames)
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
			log.Fatal().Err(err).Msgf("We had an issue locating the %v file.", fileReg)
		} else {
			if m {
				return true, file
			}
		}
	}
	return false, ""
}
