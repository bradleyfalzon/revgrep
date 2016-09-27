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

type Checker struct {
	// Unified patch file to read to detect lines being changed, if nil revgrep
	// will attempt to detect the VCS and generate an appropriate patch. Auto
	// detection will search for uncommitted changes first, if none found, will
	// generate a patch from last committed change.
	Patch io.Reader
	// Debug sets the debug writer for additional output
	Debug io.Writer
}

// Check scans reader and writes any lines to writer that have been added in
// Checker.Patch. Returns number of issues written to writer. If no VCS could
// be found or other VCS errors occur, all issues are written to writer.
func (c Checker) Check(reader io.Reader, writer io.Writer) int {

	// file:lineNumber
	lineRE := regexp.MustCompile("^(.*):([0-9]+)")

	// Check if patch is supplied, if not, retrieve from VCS
	var writeAll bool
	if c.Patch == nil {
		var err error
		c.Patch, err = GitPatch()
		if err != nil {
			writeAll = true
			c.debug("could not read git repo:", err)
		}
		if c.Patch == nil {
			writeAll = true
			c.debug("no version control repository found")
		}
	}

	// TODO consider lazy loading this, if there's nothing in stdin, no point
	// checking for recent changes
	linesChanged := c.linesChanged()
	c.debug(fmt.Sprintf("lines changed: %+v", linesChanged))

	// Scan each line in reader and only write those lines if lines changed
	issueCount := 0
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := lineRE.FindSubmatch(scanner.Bytes())
		if line == nil {
			c.debug("cannot parse file+line number:", scanner.Text())
			continue
		}

		if writeAll {
			fmt.Fprintln(writer, scanner.Text())
			continue
		}

		// Parse line number
		lno, err := strconv.ParseUint(string(line[2]), 10, 64)
		if err != nil {
			c.debug("cannot parse line number:", scanner.Text())
			continue
		}

		var changed bool
		if fchanges, ok := linesChanged[string(line[1])]; ok {
			// found file, see if lines matched
			for _, fno := range fchanges {
				if fno == lno {
					changed = true
					issueCount++
					fmt.Fprintln(writer, scanner.Text())
				}
			}
		}
		if !changed {
			c.debug("unchanged:", scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	return issueCount
}

func (c Checker) debug(s ...interface{}) {
	if c.Debug != nil {
		fmt.Fprint(c.Debug, "DEBUG: ")
		fmt.Fprintln(c.Debug, s...)
	}
}

// linesChanges returns a map of file names to line numbers being changed
func (c Checker) linesChanged() map[string][]uint64 {
	type state struct {
		file    string
		lineNo  uint64   // current line number within chunk
		changes []uint64 // line numbers being changed
	}

	var (
		s       state
		changes = make(map[string][]uint64)
	)

	if c.Patch == nil {
		return changes
	}

	scanner := bufio.NewScanner(c.Patch)
	for scanner.Scan() {
		line := scanner.Text() // TODO scanner.Bytes()
		c.debug(line)
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

	var (
		unstaged  bool
		untracked bool
	)

	// check for unstaged changes
	// use --no-prefix to remove b/ given: +++ b/main.go

	cmd := exec.Command("git", "diff", "--no-prefix")
	cmd.Stdout = &patch
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error executing git diff: %s", err)
	}
	unstaged = patch.Len() > 0

	// make a patch from untracked files

	ls, err := exec.Command("git", "ls-files", "-o").CombinedOutput()
	for _, file := range bytes.Split(ls, []byte{'\n'}) {
		if len(file) == 0 {
			continue
		}
		makePatch(string(file), &patch)
		untracked = true
	}

	// If git diff show unstaged changes, use that patch
	if unstaged || untracked {
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
