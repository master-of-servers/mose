package utils

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// https://www.systutorials.com/241704/how-to-get-the-hostname-of-the-node-in-go/
func GetHostname() string {
	name, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	return name
}

/*
Found in:
https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go/23558495
*/
func GetLocalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
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
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

/*
Based on ideas found in https://stackoverflow.com/questions/39320025/how-to-stop-http-listenandserve
and https://hackernoon.com/how-to-create-a-web-server-in-go-a064277287c9
*/
func StartHttpServer(port int, webDir string) *http.Server {
	srv := &http.Server{Addr: ":" + strconv.Itoa(port)}

	if webDir != "" {
		http.Handle("/", http.FileServer(http.Dir(webDir)))
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %s", err)
		}
	}()

	return srv
}

func Get() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}

	addrs, err := net.LookupIP(hostname)
	if err != nil {
		return hostname
	}

	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			ip, err := ipv4.MarshalText()
			if err != nil {
				return hostname
			}
			hosts, err := net.LookupAddr(string(ip))
			if err != nil || len(hosts) == 0 {
				return hostname
			}
			fqdn := hosts[0]
			return strings.TrimSuffix(fqdn, ".") // return fqdn without trailing dot
		}
	}
	return hostname
}
