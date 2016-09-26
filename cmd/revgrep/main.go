package main

import (
	"os"

	"github.com/bradleyfalzon/revgrep"
)

func main() {
	// Get lines changes
	revgrep.Changes(nil, os.Stdin, os.Stderr)
}
