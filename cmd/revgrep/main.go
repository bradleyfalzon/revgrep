package main

import (
	"flag"
	"os"

	"github.com/bradleyfalzon/revgrep"
)

func main() {
	debug := flag.Bool("d", false, "Debug")
	flag.Parse()

	var checker revgrep.Checker

	if *debug {
		checker.Debug = os.Stdout
	}

	issueCount := checker.Check(os.Stdin, os.Stderr)
	if issueCount > 0 {
		os.Exit(1)
	}
}
