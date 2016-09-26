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

// Changes scans reader and writes any lines to writer that have been added
// in patch, if patch is nil, Changes will detect version control repository
// and generate a suitable patch. Returns number of issues written to writer.
// If no VCS could be found or other VCS errors occur, all issues are written
// to writer.
func Changes(patch io.Reader, reader io.Reader, writer io.Writer) int {

	// file:lineNumber
	lineRE := regexp.MustCompile("^(.*):([0-9]+)")

	var writeAll bool
	if patch == nil {
		var err error
		patch, err = GitPatch()
		if err != nil {
			writeAll = true
			fmt.Fprintf(os.Stderr, "Could not read git repo: %v\n", err)
		}
		if patch == nil {
			writeAll = true
			fmt.Fprintln(os.Stderr, "No version control repository found")
		}
	}

	// TODO consider lazy loading this, if there's nothing in stdin, no point
	// checking for recent changes
	linesChanged := linesChanged(patch)

	issueCount := 0
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := lineRE.FindSubmatch(scanner.Bytes())
		if line == nil {
			continue
		}

		if writeAll {
			fmt.Fprintln(writer, scanner.Text())
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
					issueCount++
					fmt.Fprintln(writer, scanner.Text())
				}
			}
		}

	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	return issueCount
}

// linesChanges returns a map of file names to line numbers being changed
func linesChanged(patch io.Reader) map[string][]uint64 {
	type state struct {
		file    string
		lineNo  uint64   // current line number within chunk
		changes []uint64 // line numbers being changed
	}

	var (
		s       state
		changes = make(map[string][]uint64)
	)

	if patch == nil {
		return changes
	}

	scanner := bufio.NewScanner(patch)
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
	foundUnstaged := patch.Len() > 0

	// make a patch from untracked files

	ls, err := exec.Command("git", "ls-files", "-o").CombinedOutput()
	for _, file := range bytes.Split(ls, []byte{'\n'}) {
		if len(file) == 0 {
			continue
		}
		makePatch(string(file), &patch)
	}

	// If git diff show unstaged changes, use that patch
	if foundUnstaged {
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

// makePatch makes a patch from a file on the file system, writes to patch
// TODO this shouldn't require an external dependency and could be refactored
// into a different method
func makePatch(file string, patch io.Writer) {
	cmd := exec.Command("diff", "-u", os.DevNull, file)
	cmd.Stdout = patch
	// ignore errors from cmd.Run(), this maybe an untracked due to binary file
	_ = cmd.Run()
}
