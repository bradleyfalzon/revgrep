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
		fmt.Println("If no revisions are given, and there are unstaged changes or untracked files, only those changes are shown")
		fmt.Println("If no revisions are given, and there are no unstaged changes or untracked files, only changes in HEAD~ are shown")
		fmt.Println("If from-rev is given and to-rev is not, only changes between from-rev and HEAD are shown.")
		fmt.Println()
		flag.PrintDefaults()
	}

	debug := flag.Bool("d", false, "Show debug output")
	regexp := flag.String("regexp", "", "Regexp to match path, line number, optional column number, and message")
	flag.Parse()

	checker := revgrep.Checker{
		RevisionFrom: flag.Arg(0),
		RevisionTo:   flag.Arg(1),
		Regexp:       *regexp,
	}

	if *debug {
		checker.Debug = os.Stdout
	}

	issues, err := checker.Check(os.Stdin, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(issues) > 0 {
		os.Exit(1)
	}
}
