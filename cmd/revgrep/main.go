package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bradleyfalzon/revgrep"
)

func main() {
	flag.Usage = func() {
		fmt.Println("Usage: revgrep [options] [from-rev] [to-rev]")
		fmt.Println()
		fmt.Println("from-rev filters issues to lines changed since (and including) this revision")
		fmt.Println("  to-rev filters issues to lines changed since (and including) this revision, requires <from-rev>")
		fmt.Println()
		flag.PrintDefaults()
	}

	debug := flag.Bool("d", false, "Show debug output")
	flag.Parse()

	checker := revgrep.Checker{
		RevisionFrom: flag.Arg(0),
		RevisionTo:   flag.Arg(1),
	}

	if *debug {
		checker.Debug = os.Stdout
	}

	issueCount := checker.Check(os.Stdin, os.Stderr)
	if issueCount > 0 {
		os.Exit(1)
	}
}
