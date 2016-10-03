package main

import (
	"flag"
	"os"

	"github.com/bradleyfalzon/revgrep"
)

func main() {
	debug := flag.Bool("d", false, "Debug")
	revFrom := flag.String("from", "", "Filter issues to lines changed since (and including) this revision")
	revTo := flag.String("to", "", "Filter issues to lines changed since (and including) this revision (requires -from to be set)")
	flag.Parse()

	checker := revgrep.Checker{
		RevisionFrom: *revFrom,
		RevisionTo:   *revTo,
	}

	if *debug {
		checker.Debug = os.Stdout
	}

	issueCount := checker.Check(os.Stdin, os.Stderr)
	if issueCount > 0 {
		os.Exit(1)
	}
}
