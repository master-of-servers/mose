// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
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
				strings.Replace(text, "\r\n", "", -1)
			} else {
				strings.Replace(text, "\n", "", -1)
			}
			return true, nil
		} else if strings.Contains(text, "q") {
			return false, errors.New("Quit")
		} else if strings.Contains(text, "n") {
			return false, nil
		} else {
			ErrMsg("Invalid input")
		}
	}
}

// IndexedUserQuestion takes a question from a user and returns true or false based on the input.
// The input must be a number, which correspond to an index of an answer.
// The operating system must be specified as an input in order to handle the line ending properly;
// Windows uses a different line ending scheme than Unix systems
// pp is used to pass in an anonymous function for pretty printing - see validateIndicies() in cmd/mose/ansible/main.go for an example
// Loosely based on https://tutorialedge.net/golang/reading-console-input-golang/
func IndexedUserQuestion(question string, osTarget string, validIndices map[int]bool, pp func()) (map[int]bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		var err error
		fmt.Println(question)
		text, _ := reader.ReadString('\n')
		if strings.Contains(text, "q") {
			return nil, errors.New("Quit")
		}
		if osTarget == "windows" {
			text = text[:len(text)-2]
		} else {
			text = text[:len(text)-1]
		}
		strnums := strings.Split(text, ",")
		nums := make(map[int]bool)
		for _, n := range strnums {
			n = strings.TrimSpace(n)
			num, e := strconv.Atoi(n)
			if e != nil {
				ErrMsg("Number provided is not an integer")
				err = e
			} else if !validIndices[num] {
				ErrMsg("Number is not valid, try again")
				if pp != nil {
					pp()
				}
				err = errors.New("input number is not a valid index")
			} else {
				nums[num] = true
			}
		}
		if err == nil && len(nums) > 0 {
			return nums, nil
		}
	}
}
