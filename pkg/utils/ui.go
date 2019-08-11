package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

// AskUserQuestion takes a question from a user and returns true or false based on the input.
// The operating system must be specified as an input in order to handle the line ending properly;
// Windows uses a different line ending scheme than Unix systems
// Loosely based on https://tutorialedge.net/golang/reading-console-input-golang/
func AskUserQuestion(question string, osTarget string) bool {
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
			return true
		} else if strings.Contains(text, "q") {
			log.Fatal("Exiting...")
		} else if strings.Contains(text, "n") {
			return false
		} else {
			log.Println("Invalid input")
		}
	}
	return false
}
