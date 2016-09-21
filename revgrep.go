package revgrep

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type state struct {
	file    string
	cstart  uint64   // chunkstart == 2 given: @@ -1 +2,4 @@
	lineNo  uint64   // current line number within chunk
	changes []uint64 // line numbers being changes
}

func Changes(reader io.Reader, writer io.Writer) {

	fmt.Println("Checking for changes...")

	// file:lineNumber
	lineRE := regexp.MustCompile("^(.*):([0-9]+).*")

	// TODO consider lazy loading this, if there's nothing in stdin, no point
	// checking for recent changes
	linesChanged := linesChanged()
	fmt.Printf("changed lines: %+v\n", linesChanged)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := lineRE.FindSubmatch(scanner.Bytes())
		if line == nil {
			continue
		}

		// Parse line number
		lno, err := strconv.ParseUint(string(line[2]), 10, 64)
		if err != nil {
			continue
		}

		if fchanges, ok := linesChanged[string(line[1])]; ok {
			// found file, see if lines matched
			for _, fno := range fchanges {
				if fno == lno {
					fmt.Fprintf(writer, "%s\n", line[0])
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

// linesChanges returns a map of file names to line numbers being changed
func linesChanged() map[string][]uint64 {
	// TODO returned file name should be full filesystem path

	// --no-prefix to remove b/ given: +++ b/main.go
	diff, err := exec.Command("git", "diff", "--no-prefix").CombinedOutput()
	if err != nil {
		panic(err)
	}
	// TODO stream this
	scanner := bufio.NewScanner(bytes.NewBuffer(diff))
	var (
		s       state
		changes = make(map[string][]uint64)
	)
	for scanner.Scan() {
		line := scanner.Text() // TODO scanner.Bytes()
		s.lineNo++
		switch {
		case len(line) >= 4 && line[:3] == "+++":
			if s.changes != nil {
				// record the last state
				changes[s.file] = s.changes
			}
			s = state{file: line[4:]}
		case len(line) >= 3 && line[:3] == "@@ ":
			//      @@ -1 +2,4 @@
			// chdr ^^^^^^^^^^^^^
			// ahdr       ^^^^
			// cstart      ^
			chdr := strings.Split(line, " ")
			ahdr := strings.Split(chdr[2], ",")
			var err error
			// [1:] to remove leading plus
			s.cstart, err = strconv.ParseUint(ahdr[0][1:], 10, 64)
			if err != nil {
				panic(err)
			}
		case len(line) > 0 && line[:1] == "+":
			s.changes = append(s.changes, s.lineNo)
		}

	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	// record the last state
	changes[s.file] = s.changes

	return changes
}
