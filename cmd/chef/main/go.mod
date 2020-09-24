module main

go 1.14

require (
	github.com/markbates/pkger v0.17.1
	github.com/master-of-servers/mose v1.2.5
	github.com/rs/zerolog v1.20.0
)

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20190423201726-d2cfbce3f3b0

replace github.com/master-of-servers/mose => ../../../
