// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

// AskUserQuestion takes a question from a user and returns true or false based on the input.
// The operating system must be specified as an input in order to handle the line ending properly;
// Windows uses a different line ending scheme than Unix systems
// Loosely based on https://tutorialedge.net/golang/reading-console-input-golang/
func AskUserQuestion(question string, osTarget string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println(question + "[Y/n/q]")
		text, _ := reader.ReadString('\n')
		if strings.Contains(text, "Y") {
			if osTarget == "windows" {
				text = strings.Replace(text, "\r\n", "", -1)
			} else {
				text = strings.Replace(text, "\n", "", -1)
			}
			return true, nil
		} else if strings.Contains(text, "q") {
			return false, errors.New("Quit")
		} else if strings.Contains(text, "n") {
			return false, nil
		} else {
			log.Println("Invalid input")
		}
	}
}
