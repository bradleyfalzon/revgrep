package main

import (
	"fmt"
	"os"

	"github.com/bradleyfalzon/revgrep"
)

func main() {
	fmt.Println("Starting...")

	// Get lines changes
	revgrep.Changes(nil, os.Stdin, os.Stderr)

	// Open stdin and scan

	// Check if line was affected

}
