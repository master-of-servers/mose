module main

go 1.14

require (
	github.com/markbates/pkger v0.17.0
	github.com/master-of-servers/mose v1.2.5
	github.com/rs/zerolog v1.19.0
	github.com/ulikunitz/xz v0.5.8 // indirect
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	golang.org/x/sys v0.0.0-20200831180312-196b9ba8737a // indirect
)

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20190423201726-d2cfbce3f3b0

replace github.com/master-of-servers/mose => ../../../
