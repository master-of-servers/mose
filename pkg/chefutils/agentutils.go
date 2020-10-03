// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package chefutils

import (
	"bufio"
	"errors"
	"os"
	"strings"

	"github.com/master-of-servers/mose/pkg/moseutils"
)

// TargetAgents allows a user to select specific chef agents, or return them all as a []string
func TargetAgents(nodes []string, osTarget string) ([]string, error) {
	var targets []string
	if ans, err := moseutils.AskUserQuestion("Do you want to target specific chef agents? ", osTarget); ans && err == nil {
		reader := bufio.NewReader(os.Stdin)
		// Print the first discovered node (done for formatting purposes)
		// Print the rest of the discovered nodes
		validAgents := make(map[string]bool)
		printNodes := func() {
			for _, node := range nodes {
				if node != "" {
					moseutils.ColorMsgf("%s", node)
					if !validAgents[node] {
						validAgents[node] = true
					}
				}
			}
		}
	Validated:
		for {
			printNodes()
			moseutils.ColorMsgf("Please input the chef agents that you want to target using commas to separate them: ")
			text, _ := reader.ReadString('\n')
			targets = strings.Split(strings.TrimSuffix(text, "\n"), ",")
			for ind, uTarget := range targets {
				switch found := validAgents[uTarget]; found {
				case true:
					if ind == len(targets)-1 {
						break Validated
					}
				case false:
					break
				}
			}
		}
	} else if !ans && err == nil {
		// Target all of the agents
		return []string{"MOSEALL"}, nil
	} else if err != nil {
		return nil, errors.New("Quit")
	}
	return targets, nil
}
