package main

import (
	"os"

	"github.com/bradleyfalzon/revgrep"
)

func main() {
	// Get lines changes
	issueCount := revgrep.Changes(nil, os.Stdin, os.Stderr)
	if issueCount > 0 {
		os.Exit(1)
	}
}
