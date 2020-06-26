// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package userinput

import (
	"context"
	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/netutils"
	"github.com/master-of-servers/mose/pkg/system"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/markbates/pkger"
)

type UserInput struct {
	// CLI
	OSArch               string `mapstructure:"osarch,omitempty"`
	Cmd                  string `mapstructure:"cmd,omitempty"`
	Debug                bool   `mapstructure:"debug,omitempty"`
	ExfilPort            int    `mapstructure:"exfilport,omitempty"`
	FilePath             string `mapstructure:"filepath,omitempty"`
	FileUpload           string `mapstructure:"fileupload,omitempty"`
	LocalIP              string `mapstructure:"localip,omitempty"`
	PayloadName          string `mapstructure:"payloadname,omitempty"`
	NoServe              bool   `mapstructure:"noserve,omitempty"`
	OSTarget             string `mapstructure:"ostarget,omitempty"`
	WebSrvPort           int    `mapstructure:"websrvport,omitempty"`
	RemoteUploadFilePath string `mapstructure:"remoteuploadpath,omitempty"`
	Rhost                string `mapstructure:"rhost,omitempty"`
	ServeSSL             bool   `mapstructure:"ssl,omitempty"`
	TimeToServe          int    `mapstructure:"tts,omitempty"`
	// Settings
	AnsibleBackupLoc    string `mapstructure:"AnsibleBackupLoc,omitempty"`
	ChefClientKey       string `mapstructure:"ChefClientKey,omitempty"`
	ChefNodeName        string `mapstructure:"ChefNodeName,omitempty"`
	ChefValidationKey   string `mapstructure:"ChefValidationKey,omitempty"`
	CleanupFile         string `mapstructure:"CleanupFile,omitempty"`
	ContainerName       string `mapstructure:"ContainerName,omitempty"`
	ImageName           string `mapstructure:"ImageName,omitempty"`
	PuppetBackupLoc     string `mapstructure:"PuppetBackupLoc,omitempty"`
	RemoteHost          string `mapstructure:"RemoteHost,omitempty"`
	SaltBackupLoc       string `mapstructure:"SaltBackupLoc,omitempty"`
	SSLCertPath         string `mapstructure:"SSLCertPath,omitempty"`
	SSLKeyPath          string `mapstructure:"SSLKeyPath,omitempty"`
	TargetChefServer    string `mapstructure:"TargetChefServer,omitempty"`
	TargetOrgName       string `mapstructure:"TargetOrgName,omitempty"`
	TargetValidatorName string `mapstructure:"TargetValidatorName,omitempty"`
	UploadFilePath      string `mapstructure:"UploadFilePath,omitempty"`
	PayloadDirectory    string `mapstructure:"payloads,omitempty"`

	CMTarget string `mapstructure:"cmtarget,omitempty"`
	BaseDir  string `mapstructure:"basedir,omitempty"`
	NoColor  bool   `mapstructure:"nocolor,omitempty"`
}

func (i *UserInput) StartTakeover() {
	// Output to the payloads directory if -f is specified
	if i.FileUpload != "" {
		targetBin := filepath.Join(i.PayloadDirectory, i.CMTarget+"-"+strings.ToLower(i.OSTarget))
		files := []string{filepath.Join(i.PayloadDirectory, filepath.Base(i.FileUpload)), targetBin}
		archiveLoc := filepath.Join(i.PayloadDirectory, "/files.tar")
		if i.FilePath != "" {
			archiveLoc = i.FilePath
		}

		// Specify tar for the archive type if no extension is defined
		if filepath.Ext(archiveLoc) == "" {
			archiveLoc = archiveLoc + ".tar"
		}

		log.Info().Msgf("Compressing files %v into %s", files, archiveLoc)

		loc, err := system.ArchiveFiles(files, archiveLoc)
		if err != nil {
			log.Error().Err(err).Msg("Error generating archive file")
		}
		log.Debug().Msgf("Archive file created at %s", loc)
	}

	// If the user hasn't specified to output the payload to a file, then serve it
	if i.FilePath == "" {
		i.ServePayload()
	}

}
func (i *UserInput) GenerateParams() {
	var origFileUpload string

	paramLoc := filepath.Join(i.BaseDir, "cmd/", i.CMTarget, "main/tmpl")
	//paramLoc := filepath.Join(CMTarget, "tmpl")
	//box := pkger.New("Params", "|")
	//box.ResolutionDir = paramLoc
	//pkger.Include("/cmd/mose")

	// Generate the params for a given target
	//s, err := box.FindString("params.tmpl")
	s, err := pkger.Open(filepath.Join("/", paramLoc, "params.tmpl"))

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	defer s.Close()

	dat := new(strings.Builder)
	_, err = io.Copy(dat, s)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	t, err := template.New("params").Parse(dat.String())

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	f, err := os.Create(filepath.Join(i.BaseDir, "/cmd/", i.CMTarget, "main/params.go"))
	// f, err := os.Create(CMTarget + "/params.go")

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	// Temporarily set UserInput.FileUpload to the name of the file uploaded to avoid pathing issues in the payload
	if i.FileUpload != "" {
		origFileUpload = i.FileUpload
		i.FileUpload = filepath.Base(i.FileUpload)
	}
	err = t.Execute(f, i)

	f.Close()

	if i.FileUpload != "" {
		i.FileUpload = origFileUpload
	}

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	dir, _ := path.Split(i.FilePath)

	// Check if dir exists and filePath isn't empty
	if _, err := os.Stat(dir); os.IsNotExist(err) && dir != "" && i.FilePath != "" {
		log.Fatal().Msgf("Location %s does not exist", i.FilePath)
	}

	if dir == "" && i.FilePath != "" {
		dir, err := os.Getwd()
		if err != nil {
			log.Fatal().Msg("Couldn't get current working directory")
		}

		i.FilePath = filepath.Join(dir, i.FilePath)
	}

	// Set port option
	if !i.ServeSSL && i.WebSrvPort == 443 {
		i.WebSrvPort = 8090
	}

	// Put it back
	if i.FileUpload != "" {
		i.FileUpload = origFileUpload
	}
}

func (i *UserInput) GeneratePayload() {
	if i.Cmd != "" {
		log.Info().Msgf("Generating %s payload to run %s on a %s system, please wait...", i.CMTarget, i.Cmd, strings.ToLower(i.OSTarget))
	} else {
		log.Info().Msgf("Generating %s payload to run %s on a %s system, please wait...", i.CMTarget, filepath.Base(i.FileUpload), strings.ToLower(i.OSTarget))
	}

	//prevDir := utils.Gwd()
	//moseutils.Cd(filepath.Clean(filepath.Join("cmd/github.com/master-of-servers/mose/", CMTarget)))

	_ = os.Mkdir(i.PayloadDirectory, 0755)

	payload := filepath.Join(i.PayloadDirectory, i.CMTarget+"-"+strings.ToLower(i.OSTarget))

	log.Debug().Msgf("Payload output name = %s", filepath.Base(payload))
	log.Debug().Msgf("CM Target = %s", i.CMTarget)
	log.Debug().Msgf("OS Target = %s", i.OSTarget)
	if i.FileUpload != "" {
		log.Debug().Msgf("File to upload and run = %s", i.FileUpload)
	}

	// If FileUpload is specified, we need to copy it into place
	if i.FileUpload != "" {
		err := system.CpFile(i.FileUpload, filepath.Join(i.PayloadDirectory, filepath.Base(i.FileUpload)))
		if err != nil {
			log.Fatal().Err(err).Msgf("Failed to copy input file upload (%v) exiting", i.FileUpload)
		}
	}

	// FilePath specified with command to run
	if i.FilePath != "" && i.FileUpload == "" {
		log.Info().Msgf("Creating binary at: " + i.FilePath)
		payload = i.FilePath
		if !filepath.IsAbs(i.FilePath) {
			payload = filepath.Join(i.BaseDir, i.FilePath)
		}
	}

	// FilePath used as tar output location in conjunction with FileUpload
	if i.FilePath != "" && i.FileUpload != "" {
		log.Info().Msgf("File Upload specified, copying file to payloads directory. FilePath supplied, tar file will be located at specified location")
		system.CpFile(i.FileUpload, filepath.Join(i.PayloadDirectory, filepath.Base(i.FileUpload)))
	}

	//_, err := system.RunCommand("env", "GOOS="+strings.ToLower(i.OSTarget), "GOARCH=amd64", "go", "build", "-o", payload, filepath.Join(i.BaseDir, "cmd", i.CMTarget, "main.go"), filepath.Join(i.BaseDir, "cmd", i.CMTarget, "params.go"))
	//commandString := fmt.Sprintf("GOOS=%s GOARCH=amd64 go build -o %s %s %s", strings.ToLower(i.OSTarget), payload, filepath.Join(i.BaseDir, "cmd", i.CMTarget, "main.go"), filepath.Join(i.BaseDir, "cmd", i.CMTarget, "params.go"))
	//_, err := utils.RunCommand("env", commandString)
	cmd := exec.Command("pkger")
	cmd.Dir = filepath.Join(i.BaseDir, "cmd", i.CMTarget, "main")
	err := cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Error running the command to generate the target payload")
	}

	cmd = exec.Command("env", "GOOS="+strings.ToLower(i.OSTarget), "GOARCH=amd64", "go", "build", "-o", payload)
	log.Debug().Msgf("env GOOS=%s GOARCH=amd64 go build -o %s", strings.ToLower(i.OSTarget), payload)
	cmd.Dir = filepath.Join(i.BaseDir, "cmd", i.CMTarget, "main")
	err = cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Error running the command to generate the target payload")
	}

	//moseutils.Cd(prevDir)
}

func (i *UserInput) SetLocalIP() {
	if i.LocalIP == "" {
		ip, err := netutils.GetLocalIP()
		i.LocalIP = ip
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
	}
}

func (i *UserInput) ServePayload() {
	proto := "http"
	if i.ServeSSL {
		proto = "https"
	}

	if i.FileUpload != "" {
		moseutils.ColorMsgf("File upload command specified, payload being served at %s://%s:%d/files.tar for %d seconds", proto, i.LocalIP, i.WebSrvPort, i.TimeToServe)
	} else {
		moseutils.ColorMsgf("Payload being served at %s://%s:%d/%s-%s for %d seconds", proto, i.LocalIP, i.WebSrvPort, i.CMTarget, strings.ToLower(i.OSTarget), i.TimeToServe)
	}

	//srv := moseutils.StartServer(i.WebSrvPort, "payloads", i.ServeSSL, i.SSLCertPath, i.SSLKeyPath, time.Duration(i.TimeToServe)*time.Second, true)
	srv := netutils.StartServer(i.WebSrvPort, i.PayloadDirectory, i.ServeSSL, i.SSLCertPath, i.SSLKeyPath, time.Duration(i.TimeToServe)*time.Second, true)

	log.Info().Msg("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
