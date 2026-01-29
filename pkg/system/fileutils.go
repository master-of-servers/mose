// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package system

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/rs/zerolog/log"
)

// CreateDirectories creates folders specified in an input slice (folders)
// It returns true if it is able to create all of the folders
// Otherwise it returns false
func CreateDirectories(folders []string) bool {
	for _, f := range folders {
		err := os.MkdirAll(f, os.ModePerm)
		if err != nil {
			log.Error().Err(err).Msg("")
			return false
		}
		moseutils.ColorMsgf("Creating %s directory", f)
	}
	return true
}

// FileExists returns true if a file input (fileLoc) exists on the filesystem
// Otherwise it returns false
func FileExists(fileLoc string) bool {
	_, err := os.Stat(fileLoc)
	return err == nil
}

// GrepFile looks for patterns in a file (filePath) using an input regex (regex)
// It will return any matches that are found in the file in a slice
func GrepFile(filePath string, regex *regexp.Regexp) []string {
	file, err := os.ReadFile(filePath)

	if err != nil {
		log.Error().Err(err).Msgf("Unable to read file %v", filePath)
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
	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// WriteBytesToFile writes a byte array to the file specified with filePath with the permissions specified in perm
// An error will be returned if there is one
func WriteBytesToFile(filePath string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(filePath, data, perm); err != nil {
		return fmt.Errorf("write file %s: %w", filePath, err)
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

	var builder strings.Builder
	for i, line := range lines {
		if i == index {
			builder.WriteString(str)
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return os.WriteFile(path, []byte(builder.String()), 0644)
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
// currently only supports tar, tar.gz, tgz, and zip based archives.
func ArchiveFiles(files []string, archiveLocation string) (string, error) {
	if _, err := os.Stat(archiveLocation); !os.IsNotExist(err) {
		_ = os.Remove(archiveLocation)
	}

	lowerPath := strings.ToLower(archiveLocation)
	switch {
	case strings.HasSuffix(lowerPath, ".tar.gz") || strings.HasSuffix(lowerPath, ".tgz"):
		if err := archiveTar(files, archiveLocation, true); err != nil {
			return "", err
		}
	case strings.HasSuffix(lowerPath, ".tar"):
		if err := archiveTar(files, archiveLocation, false); err != nil {
			return "", err
		}
	case strings.HasSuffix(lowerPath, ".zip"):
		if err := archiveZip(files, archiveLocation); err != nil {
			return "", err
		}
	default:
		return "", errors.New("the following archive types are not supported: tar.gz, tgz, tar, zip")
	}
	return archiveLocation, nil
}

func archiveTar(files []string, archiveLocation string, gzipEnabled bool) error {
	outFile, err := os.Create(archiveLocation)
	if err != nil {
		return err
	}
	defer func() {
		_ = outFile.Close()
	}()

	var writer io.WriteCloser = outFile
	var gw *gzip.Writer
	if gzipEnabled {
		gw = gzip.NewWriter(outFile)
		writer = gw
	}

	tw := tar.NewWriter(writer)

	for _, root := range files {
		if err := addPathToTar(tw, root); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if gw != nil {
		if err := gw.Close(); err != nil {
			return err
		}
	}
	if err := outFile.Close(); err != nil {
		return err
	}
	return nil
}

func addPathToTar(tw *tar.Writer, root string) error {
	baseDir := filepath.Dir(root)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "/") || rel == "." {
			return fmt.Errorf("invalid archive path: %s", rel)
		}

		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = rel
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, file); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
		return nil
	})
}

func archiveZip(files []string, archiveLocation string) error {
	outFile, err := os.Create(archiveLocation)
	if err != nil {
		return err
	}
	defer func() {
		_ = outFile.Close()
	}()

	zw := zip.NewWriter(outFile)

	for _, root := range files {
		if err := addPathToZip(zw, root); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := outFile.Close(); err != nil {
		return err
	}
	return nil
}

func addPathToZip(zw *zip.Writer, root string) error {
	baseDir := filepath.Dir(root)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "/") || rel == "." {
			return fmt.Errorf("invalid archive path: %s", rel)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(writer, file); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplLineInFile will replace a line in a file (filePath) with the specified replStr and delimiter (delim)
// It will return true with the path to the file if successful
// Otherwise it will return false and an empty string
func ReplLineInFile(filePath string, delim string, replStr string) (bool, string) {
	input, err := os.ReadFile(filePath)
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
	err = os.WriteFile(filePath, []byte(output), 0644)
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
	foundFiles := make(map[string]struct{})
	foundDirs := make(map[string]struct{})
	fileList, dirList := GetFileAndDirList(locations)
	collectFilesByName(fileList, fileNames, foundFiles)
	collectFilesByExtension(fileList, extensionList, foundFiles)
	collectDirsByPattern(dirList, dirNames, foundDirs)

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

func collectFilesByName(fileList []string, fileNames []string, foundFiles map[string]struct{}) {
	for _, fileContains := range fileNames {
		for _, file := range fileList {
			if strings.Contains(file, fileContains) {
				foundFiles[file] = struct{}{}
			}
		}
	}
}

func collectFilesByExtension(fileList []string, extensionList []string, foundFiles map[string]struct{}) {
	for _, ext := range extensionList {
		for _, file := range fileList {
			if strings.HasSuffix(file, ext) {
				foundFiles[file] = struct{}{}
			}
		}
	}
}

func collectDirsByPattern(dirList []string, dirNames []string, foundDirs map[string]struct{}) {
	for _, reg := range dirNames {
		for _, dir := range dirList {
			m, err := regexp.MatchString(reg, dir)
			if err != nil {
				log.Fatal().Err(err).Msgf("Unable to locate the %s directory", dir)
			} else if m {
				foundDirs[dir] = struct{}{}
			}
		}
	}
}

// FindFile locates a file (fileName) in a list of input directories (dir)
// If the file is found, then it returns true along with the file location
// Otherwise it returns false with an empty string
func FindFile(fileName string, dirs []string) (bool, string) {
	fileList, _ := GetFileAndDirList(dirs)
	for _, file := range fileList {
		if filepath.Base(file) == fileName {
			return true, file
		}
	}
	return false, ""
}
