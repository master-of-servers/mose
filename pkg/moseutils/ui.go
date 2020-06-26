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

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	ColorBlack = iota + 30
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite

	ColorBold     = 1
	ColorDarkGray = 90
)

var NOCOLOR bool

// AskUserQuestion takes a question from a user and returns true or false based on the input.
// The operating system must be specified as an input in order to handle the line ending properly;
// Windows uses a different line ending scheme than Unix systems
// Loosely based on https://tutorialedge.net/golang/reading-console-input-golang/
func AskUserQuestion(question string, osTarget string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		//log.Log().Msgf(question + "[Y/n/q]")
		ColorMsgf(question + "[Y/n/q]")
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
			log.Error().Msg("Invalid input")
		}
	}
}

// IndexedUserQuestion takes a question from a user and returns true or false based on the input.
// The input must be a number, which correspond to an index of an answer.
// The operating system must be specified as an input in order to handle the line ending properly;
// Windows uses a different line ending scheme than Unix systems
// pp is used to pass in an anonymous function for pretty printing - see validateIndicies() in cmd/github.com/master-of-servers/mose/ansible/main.go for an example
// Loosely based on https://tutorialedge.net/golang/reading-console-input-golang/
func IndexedUserQuestion(question string, osTarget string, validIndices map[int]bool, pp func()) (map[int]bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		var err error
		//log.Log().Msg(question)
		ColorMsgf(question)
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
				log.Error().Msg("Number provided is not an integer")
				err = e
			} else if !validIndices[num] {
				log.Error().Msg("Number is not valid, try again")
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

func ColorMsgf(s string, i ...interface{}) {
	if NOCOLOR {
		if len(i) > 0 {
			log.Log().Msgf(s, i...)
			return
		}
		log.Log().Msg(s)
		return
	}
	if len(i) > 0 {
		log.Log().Msgf(fmt.Sprintf("\x1b[%dm%v\x1b[0m", ColorGreen, s), i...)
		return
	}
	log.Log().Msg(fmt.Sprintf("\x1b[%dm%v\x1b[0m", ColorGreen, s))
}

// colorize returns the string s wrapped in ANSI code c, unless disabled is true.
func Colorizer(s interface{}, c int, disabled bool) string {
	if disabled {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}

func SetupLogger(debug bool) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	output := zerolog.ConsoleWriter{Out: os.Stderr}
	output.FormatLevel = func(i interface{}) string {
		var l string
		if ll, ok := i.(string); ok {
			switch ll {
			case "trace":
				l = Colorizer("TRC", ColorCyan, NOCOLOR)
			case "debug":
				l = Colorizer("DBG", ColorDarkGray, NOCOLOR)
			case "info":
				l = Colorizer("INF", ColorMagenta, NOCOLOR)
			case "warn":
				l = Colorizer("WRN", ColorYellow, NOCOLOR)
			case "error":
				l = Colorizer(Colorizer("ERR", ColorRed, NOCOLOR), ColorBold, NOCOLOR)
			case "fatal":
				l = Colorizer(Colorizer("FTL", ColorRed, NOCOLOR), ColorBold, NOCOLOR)
			case "panic":
				l = Colorizer(Colorizer("PNC", ColorRed, NOCOLOR), ColorBold, NOCOLOR)
			default:
				l = Colorizer(Colorizer("MSG", ColorGreen, NOCOLOR), ColorBold, NOCOLOR)
			}
		} else {
			if i == nil {
				l = Colorizer(Colorizer("MSG", ColorGreen, NOCOLOR), ColorBold, NOCOLOR)
			} else {
				l = strings.ToUpper(fmt.Sprintf("%s", i))[0:3]
			}
		}
		return l
		//return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}
	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf(": %s", i)
	}
	output.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}
	output.FormatFieldValue = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("%s", i))
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}
