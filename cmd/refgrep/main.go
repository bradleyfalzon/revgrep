package main

import (
	"fmt"
	"os"

	"github.com/bradleyfalzon/refgrep"
)

func main() {
	fmt.Println("Starting...")

	// Get lines changes
	refgrep.Changes(os.Stdin)

	// Open stdin and scan

	// Check if line was affected

}
