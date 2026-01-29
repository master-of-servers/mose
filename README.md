# MOSE (Master Of SErvers)

[![DEF CON 27](https://img.shields.io/badge/DEF%20CON-27-green)](https://defcon.org/html/defcon-27/dc-27-speakers.html#Grace)
[![License](https://img.shields.io/github/license/master-of-servers/mose?label=License&style=flat&color=blue&logo=github)](https://github.com/master-of-servers/mose/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/master-of-servers/mose)](https://goreportcard.com/report/github.com/master-of-servers/mose)
[![Tests](https://img.shields.io/github/actions/workflow/status/master-of-servers/mose/tests.yaml)](https://github.com/master-of-servers/mose/actions/workflows/tests.yaml)
[![Pre-Commit](https://img.shields.io/github/actions/workflow/status/master-of-servers/mose/pre-commit.yaml)](https://github.com/master-of-servers/mose/actions/workflows/pre-commit.yaml)
[![Semgrep](https://img.shields.io/github/actions/workflow/status/master-of-servers/mose/semgrep.yaml)](https://github.com/master-of-servers/mose/actions/workflows/semgrep.yaml)
[![Renovate](https://img.shields.io/github/actions/workflow/status/master-of-servers/mose/renovate.yaml)](https://github.com/master-of-servers/mose/actions/workflows/renovate.yaml)
[![GoReleaser](https://img.shields.io/github/actions/workflow/status/master-of-servers/mose/goreleaser.yaml)](https://github.com/master-of-servers/mose/actions/workflows/goreleaser.yaml)

> Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
Under the terms of Contract DE-NA0003525 with NTESS,
the U.S. Government retains certain rights in this software

MOSE is a post-exploitation tool that helps security professionals leverage
configuration management (CM) systems after compromise. CM tools like
[Puppet](https://puppet.com/), [Chef](https://www.chef.io/),
[Salt](https://saltproject.io/), and [Ansible](https://www.ansible.com/)
can run commands across large fleets, but their DSLs and workflows are often
slow to learn under pressure. MOSE lets you describe what you want to execute,
then builds the CM-specific payloads for you.

## What MOSE does

1. **Abstracts CM-specific payloads** so you can focus on intent rather than
   tool-specific syntax.
1. **Targets subsets or entire inventories** for precise operations.
1. **Automates payload generation and staging** to reduce operator overhead.

## Supported CM targets

- Puppet
- Chef
- Salt
- Ansible

## MOSE + Puppet

![MOSE with Puppet](docs/images/mose_and_puppet.gif)

## MOSE + Chef

![MOSE with Chef](docs/images/mose_and_chef.gif)

## Dependencies

Install the following:

- [Golang](https://golang.org/) - tested with 1.12.7 through 1.15.2
  - Be sure to properly set your GOROOT, PATH, and GOPATH env vars.

- [Docker](https://docs.docker.com/install/) - tested with 18.09.2 through 19.03.12

## Getting started

### Install from source

Grab the code without having to clone the repo:

```bash
go get -u -v github.com/master-of-servers/mose
```

Install all go-specific dependencies and build the binary (be sure to `cd`
into the repo before running this):

```bash
make build
```

### Usage

```text
Usage:
  github.com/master-of-servers/mose [command]

Available Commands:
  ansible     Create MOSE payload for ansible
  chef        Create MOSE payload for chef
  help        Help about any command
  puppet      Create MOSE payload for puppet
  salt        Create MOSE payload for salt

Flags:
      --basedir string            Location of payloads output by mose
                                 (default "/Users/l/programs/go/src/github.com/master-of-servers/mose")
  -c, --cmd string                Command to run on the targets
      --config string             config file (default is $PWD/.settings.yaml)
      --debug                     Display debug output
      --exfilport int             Port used to exfil data from chef server
                                 (default 9090, 443 with SSL) (default 9090)
  -f, --filepath string           Output binary locally at <filepath>
  -u, --fileupload string         File upload option
  -h, --help                      help for github.com/master-of-servers/mose
  -l, --localip string            Local IP Address
      --nocolor                   Disable colors for mose
  -a, --osarch string             Architecture that the target CM tool is running on
  -o, --ostarget string           Operating system that the target CM server is on (default "linux")
  -m, --payloadname string        Name for backdoor payload (default "my_cmd")
      --payloads string           Location of payloads output by mose
                                 (default "/Users/l/programs/go/src/github.com/master-of-servers/mose/payloads")
      --remoteuploadpath string   Remote file path to upload a script to
                                 (used in conjunction with -fu)
                                 (default "/root/.definitelynotevil")
  -r, --rhost string              Set the remote host for /etc/hosts in the chef workstation container (format is hostname:ip)
      --ssl                       Serve payload over TLS
      --tts int                   Number of seconds to serve the payload (default 60)
      --websrvport int            Port used to serve payloads
                                 (default 8090, 443 with SSL) (default 8090)

Use "github.com/master-of-servers/mose [command] --help" for more information about a command.
```

### TLS Certificates

#### Recommendation

Generate and use a TLS certificate signed by a trusted Certificate Authority.

A self-signed certificate and key are provided for you, although you really
shouldn't use them. This key and certificate are widely distributed, so you can
not expect privacy if you do choose to use them. They can be found in the `data`
directory.

### Examples

You can find some examples of how to run MOSE in [EXAMPLES.md](EXAMPLES.md).

### Test Labs

Test labs that can be run with MOSE are at these locations:

- https://github.com/master-of-servers/puppet-test-lab
- https://github.com/master-of-servers/chef-test-lab
- https://github.com/master-of-servers/ansible-test-lab
- https://github.com/master-of-servers/salt-test-lab

### Responsible Use

MOSE is intended for authorized security testing and research. Ensure you have
explicit permission to operate against any environment.

### Credits

The following resources were used to help motivate the creation of this project:

- https://oneplus-x.github.io/2017/06/11/Enterprise-Offense-IT-Operations-Part-1/
- https://www.chef.io/blog/detecting-repairing-shellshock-with-chef
