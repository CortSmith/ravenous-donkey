// Command envcheck greets you and, with -v, reports the Go environment it
// runs in.
//
//	go run ./modules/00-setup/project/cmd/envcheck -v -name Ada
package main

import (
	"os"

	"gocourse/modules/00-setup/project/envcheck"
)

func main() {
	os.Exit(envcheck.Run(os.Args[1:], envcheck.DetectEnv(), os.Stdout, os.Stderr))
}
