package utils

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// https://opensource.com/article/18/6/copying-files-go
func CpFile(src string, dst string) {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		log.Println(err)
		return
	}

	err = ioutil.WriteFile(dst, input, 0644)
	if err != nil {
		log.Println("Error creating", dst)
		log.Println(err)
		return
	}
}

func Cd(dir string) {
	err := os.Chdir(dir)
	if err != nil {
		log.Fatalln(err)
	}
}

func CheckRoot() {
	if os.Geteuid() != 0 {
		log.Fatalln("This script must be run as root.")
	}
}

func Gwd() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func RunCommand(cmd string) string {
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Println(err)
		return ""
	}
	return string(out)
}
