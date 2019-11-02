// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// TargetAgents allows a user to select specific chef agents, or return them all as a []string
func TargetAgents(nodes []string, osTarget string) ([]string, error) {
	var targets []string
	if ans, err := AskUserQuestion("Do you want to target specific chef agents?", osTarget); ans && err == nil {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("%s", nodes[0])
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
