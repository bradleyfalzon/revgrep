package revgrep

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Checker struct {
	// Patch file (unified) to read to detect lines being changed, if nil revgrep
	// will attempt to detect the VCS and generate an appropriate patch. Auto
	// detection will search for uncommitted changes first, if none found, will
	// generate a patch from last committed change. File paths within patches
	// must be relative to current working directory.
	Patch io.Reader
	// NewFiles is a list of file names (with absolute paths) where the entire
	// contents of the file is new
	NewFiles []string
	// Debug sets the debug writer for additional output
	Debug io.Writer
	// RevisionFrom check revision starting at, leave blank for auto detection
	// ignored if patch is set
	RevisionFrom string
	// RevisionTo checks revision finishing at, leave blank for auto detection
	// ignored if patch is set
	RevisionTo string
}

var (
	// file:lineNumber
	lineRE = regexp.MustCompile("^(.*):([0-9]+)")
)

// Check scans reader and writes any lines to writer that have been added in
// Checker.Patch. Returns number of issues written to writer. If no VCS could
// be found or other VCS errors occur, all issues are written to writer.
// File paths in reader must be relative to current working directory or
// absolute.
func (c Checker) Check(reader io.Reader, writer io.Writer) int {
	// Check if patch is supplied, if not, retrieve from VCS
	var writeAll bool
	if c.Patch == nil {
		var err error
		c.Patch, c.NewFiles, err = GitPatch(c.RevisionFrom, c.RevisionTo)
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

	cwd, err := os.Getwd()
	if err != nil {
		c.debug(fmt.Sprintf("could not get current working directory: %s", err))
	}

	// Scan each line in reader and only write those lines if lines changed
	var issueCount int
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

		// Make absolute path names relative
		path := string(line[1])
		if rel, err := filepath.Rel(cwd, path); err == nil {
			c.debug("rewrote path from %q to %q", path, rel)
			path = rel
		}

		var changed bool
		if fchanges, ok := linesChanged[path]; ok {
			// found file, see if lines matched
			for _, fno := range fchanges {
				if fno == lno {
					changed = true
				}
			}
			if changed == true || fchanges == nil {
				issueCount++
				fmt.Fprintln(writer, scanner.Text())
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

	for _, file := range c.NewFiles {
		changes[file] = nil
	}

	if c.Patch == nil {
		return changes
	}

	scanner := bufio.NewScanner(c.Patch)
	for scanner.Scan() {
		line := scanner.Text() // TODO scanner.Bytes()
		c.debug(line)
		s.lineNo++
		switch {
		case strings.HasPrefix(line, "+++ ") && len(line) > 4:
			if s.changes != nil {
				// record the last state
				changes[s.file] = s.changes
			}
			// 6 removes "+++ b/"
			s = state{file: line[6:]}
		case strings.HasPrefix(line, "@@ "):
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
		case strings.HasPrefix(line, "-"):
			s.lineNo--
		case strings.HasPrefix(line, "+"):
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
// revisionFrom and revisionTo defines the git diff parameters, if left blank
// and there are unstaged changes or untracked files, only those will be returned
// else only check changes since HEAD~. If revisionFrom is set but revisionTo
// is not, untracked files will be included, to exclude untracked files set
// revisionTo to HEAD~. It's incorrect to specify revisionTo without a
// revisionFrom.
func GitPatch(revisionFrom, revisionTo string) (io.Reader, []string, error) {
	var patch bytes.Buffer

	// check if git repo exists
	if err := exec.Command("git", "status").Run(); err != nil {
		// don't return an error, we assume the error is not repo exists
		return nil, nil, nil
	}

	// make a patch for untracked files
	var newFiles []string
	ls, err := exec.Command("git", "ls-files", "-o").CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("error executing git ls-files: %s", err)
	}
	for _, file := range bytes.Split(ls, []byte{'\n'}) {
		if len(file) == 0 || bytes.HasSuffix(file, []byte{'/'}) {
			// ls-files was sometimes showing directories when they were ignored
			// I couldn't create a test case for this as I couldn't reproduce correctly
			// for the moment, just exclude files with trailing /
			continue
		}
		newFiles = append(newFiles, string(file))
	}

	if revisionFrom != "" {
		cmd := exec.Command("git", "diff", revisionFrom)
		if revisionTo != "" {
			cmd.Args = append(cmd.Args, revisionTo)
		}
		cmd.Stdout = &patch
		if err := cmd.Run(); err != nil {
			return nil, nil, fmt.Errorf("error executing git diff %q %q: %s", revisionFrom, revisionTo, err)
		}

		if revisionTo == "" {
			return &patch, newFiles, nil
		}
		return &patch, nil, nil
	}

	// make a patch for unstaged changes
	// use --no-prefix to remove b/ given: +++ b/main.go
	cmd := exec.Command("git", "diff")
	cmd.Stdout = &patch
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("error executing git diff: %s", err)
	}
	unstaged := patch.Len() > 0

	// If there's unstaged changes OR untracked changes (or both), then this is
	// a suitable patch
	if unstaged || newFiles != nil {
		return &patch, newFiles, nil
	}

	// check for changes in recent commit

	cmd = exec.Command("git", "diff", "HEAD~")
	cmd.Stdout = &patch
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("error executing git diff HEAD~: %s", err)
	}

	return &patch, nil, nil
}
