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
	lineNo  uint64   // current line number within chunk
	changes []uint64 // line numbers being changes
}

// Changes scans reader and writes any lines to writer that have been added
// in patch, if patch is nil, Changes will detect version control repository
// and generate a suitable patch.
func Changes(patch io.Reader, reader io.Reader, writer io.Writer) {

	// file:lineNumber
	lineRE := regexp.MustCompile("^(.*):([0-9]+).*")

	if patch == nil {
		var err error
		patch, err = GitPatch()
		if err != nil {
			panic(err)
		}
		if patch == nil {
			panic("no version control repository found")
		}
	}

	// TODO consider lazy loading this, if there's nothing in stdin, no point
	// checking for recent changes
	linesChanged := linesChanged(patch)

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
func linesChanged(patch io.Reader) map[string][]uint64 {
	// TODO returned file name should be full filesystem path
	// TODO stream this
	scanner := bufio.NewScanner(patch)
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
			// [1:] to remove leading plus
			cstart, err := strconv.ParseUint(ahdr[0][1:], 10, 64)
			if err != nil {
				panic(err)
			}
			s.lineNo = cstart - 1 // -1 as cstart is the next line number
		case len(line) > 0 && line[:1] == "-":
			s.lineNo--
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

// GitPatch returns a patch from a git repository, if no git repository was
// was found and no errors occurred, nil is returned, else an error is returned
func GitPatch() (io.Reader, error) {
	var patch bytes.Buffer

	// check if git repo exists

	err := exec.Command("git", "status").Run()
	if err != nil {
		return nil, nil
	}

	// check for unstaged changes
	// use --no-prefix to remove b/ given: +++ b/main.go

	cmd := exec.Command("git", "diff", "--no-prefix")
	cmd.Stdout = &patch
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error executing git diff: %s", err)
	}

	// If git diff show unstaged changes, use that patch
	if patch.Len() > 0 {
		return &patch, nil
	}

	// check for changes in recent commit

	cmd = exec.Command("git", "diff", "--no-prefix", "HEAD~")
	cmd.Stdout = &patch
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error executing git diff: %s", err)
	}

	return &patch, nil
}
