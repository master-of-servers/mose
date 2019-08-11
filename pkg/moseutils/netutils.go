// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	fileUploaded chan bool
)

func GetHostname() string {
	name, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	return name
}

func GetLocalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		// Interface is down
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		// Loopback interface
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			// Not an IPv4 address
			if ip == nil {
				continue
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

func singleFile(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		fileUploaded <- true
	})
}

func StartServer(port int, webDir string, ssl bool, cert string, key string, waitTime time.Duration, singleServe bool) *http.Server {
	fileUploaded = make(chan bool)
	srv := &http.Server{Addr: ":" + strconv.Itoa(port)}

	fs := http.FileServer(http.Dir(webDir))
	if webDir != "" {
		http.Handle("/", singleFile(fs))
	}

	go func() {
		var err error
		if ssl && cert != "" && key != "" {
			err = srv.ListenAndServeTLS(cert, key)
		} else {
			err = srv.ListenAndServe()
		}

		if err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %s", err)
		}
	}()
	if singleServe {

		select {
		case <-fileUploaded:
			break
		case <-time.After(waitTime):
			break
		}
	} else {
		time.Sleep(waitTime)
	}

	return srv
}
