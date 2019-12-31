// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package chefutils

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/l50/MOSE/pkg/moseutils"
)

// TargetAgents allows a user to select specific chef agents, or return them all as a []string
func TargetAgents(nodes []string, osTarget string) ([]string, error) {
	var targets []string
	if ans, err := moseutils.AskUserQuestion("Do you want to target specific chef agents? ", osTarget); ans && err == nil {
		reader := bufio.NewReader(os.Stdin)
		// Print the first discovered node (done for formatting purposes)
		fmt.Printf("%s", nodes[0])
		// Print the rest of the discovered nodes
		for _, node := range nodes[1:] {
			fmt.Printf(",%s", node)
		}
		fmt.Println("\nPlease input the chef agents that you want to target using commas to separate them: ")
		text, _ := reader.ReadString('\n')
		targets = strings.Split(strings.TrimSuffix(text, "\n"), ",")
	} else if !ans && err == nil {
		// Target all of the agents
		return []string{"MOSEALL"}, nil
	} else if err != nil {
		return nil, errors.New("Quit")
	}
	return targets, nil
}
