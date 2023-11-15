package revgrep

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Checker provides APIs to filter static analysis tools to specific commits,
// such as showing only issues since last commit.
type Checker struct {
	// Patch file (unified) to read to detect lines being changed, if nil revgrep
	// will attempt to detect the VCS and generate an appropriate patch. Auto
	// detection will search for uncommitted changes first, if none found, will
	// generate a patch from last committed change. File paths within patches
	// must be relative to current working directory.
	Patch io.Reader
	// NewFiles is a list of file names (with absolute paths) where the entire
	// contents of the file is new.
	NewFiles []string
	// Debug sets the debug writer for additional output.
	Debug io.Writer
	// RevisionFrom check revision starting at, leave blank for auto detection
	// ignored if patch is set.
	RevisionFrom string
	// RevisionTo checks revision finishing at, leave blank for auto detection
	// ignored if patch is set.
	RevisionTo string
	// Regexp to match path, line number, optional column number, and message.
	Regexp string
	// AbsPath is used to make an absolute path of an issue's filename to be
	// relative in order to match patch file. If not set, current working
	// directory is used.
	AbsPath string
}

// Issue contains metadata about an issue found.
type Issue struct {
	// File is the name of the file as it appeared from the patch.
	File string
	// LineNo is the line number of the file.
	LineNo int
	// ColNo is the column number or 0 if none could be parsed.
	ColNo int
	// HunkPos is position from file's first @@, for new files this will be the
	// line number.
	//
	// See also: https://developer.github.com/v3/pulls/comments/#create-a-comment
	HunkPos int
	// Issue text as it appeared from the tool.
	Issue string
	// Message is the issue without file name, line number and column number.
	Message string
}

// Check scans reader and writes any lines to writer that have been added in
// Checker.Patch.
//
// Returns issues written to writer when no error occurs.
//
// If no VCS could be found or other VCS errors occur, all issues are written
// to writer and an error is returned.
//
// File paths in reader must be relative to current working directory or
// absolute.
func (c Checker) Check(reader io.Reader, writer io.Writer) (issues []Issue, err error) {
	// Check if patch is supplied, if not, retrieve from VCS
	var (
		writeAll  bool
		returnErr error
	)
	if c.Patch == nil {
		c.Patch, c.NewFiles, err = GitPatch(c.RevisionFrom, c.RevisionTo)
		if err != nil {
			writeAll = true
			returnErr = fmt.Errorf("could not read git repo: %s", err)
		}
		if c.Patch == nil {
			writeAll = true
			returnErr = errors.New("no version control repository found")
		}
	}

	// file.go:lineNo:colNo:message
	// colNo is optional, strip spaces before message
	lineRE := regexp.MustCompile(`(.*?\.go):([0-9]+):([0-9]+)?:?\s*(.*)`)
	if c.Regexp != "" {
		lineRE, err = regexp.Compile(c.Regexp)
		if err != nil {
			return nil, fmt.Errorf("could not parse regexp: %v", err)
		}
	}

	// TODO consider lazy loading this, if there's nothing in stdin, no point
	// checking for recent changes
	linesChanged := c.linesChanged()
	c.debugf("lines changed: %+v", linesChanged)

	absPath := c.AbsPath
	if absPath == "" {
		absPath, err = os.Getwd()
		if err != nil {
			returnErr = fmt.Errorf("could not get current working directory: %s", err)
		}
	}

	// Scan each line in reader and only write those lines if lines changed
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := lineRE.FindSubmatch(scanner.Bytes())
		if line == nil {
			c.debugf("cannot parse file+line number: %s", scanner.Text())
			continue
		}

		if writeAll {
			fmt.Fprintln(writer, scanner.Text())
			continue
		}

		// Make absolute path names relative
		path := string(line[1])
		if rel, err := filepath.Rel(absPath, path); err == nil {
			c.debugf("rewrote path from %q to %q (absPath: %q)", path, rel, absPath)
			path = rel
		}

		// Parse line number
		lno, err := strconv.ParseUint(string(line[2]), 10, 64)
		if err != nil {
			c.debugf("cannot parse line number: %q", scanner.Text())
			continue
		}

		// Parse optional column number
		var cno uint64
		if len(line[3]) > 0 {
			cno, err = strconv.ParseUint(string(line[3]), 10, 64)
			if err != nil {
				c.debugf("cannot parse column number: %q", scanner.Text())
				// Ignore this error and continue
			}
		}

		// Extract message
		msg := string(line[4])

		c.debugf("path: %q, lineNo: %v, colNo: %v, msg: %q", path, lno, cno, msg)

		var (
			fpos    pos
			changed bool
		)
		if fchanges, ok := linesChanged[path]; ok {
			// found file, see if lines matched
			for _, pos := range fchanges {
				if pos.lineNo == int(lno) {
					fpos = pos
					changed = true
				}
			}
			if changed || fchanges == nil {
				// either file changed or it's a new file
				issue := Issue{
					File:    path,
					LineNo:  fpos.lineNo,
					ColNo:   int(cno),
					HunkPos: fpos.lineNo,
					Issue:   scanner.Text(),
					Message: msg,
				}
				if changed {
					// existing file changed
					issue.HunkPos = fpos.hunkPos
				}
				issues = append(issues, issue)
				fmt.Fprintln(writer, scanner.Text())
			}
		}
		if !changed {
			c.debugf("unchanged: %s", scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		returnErr = fmt.Errorf("error reading standard input: %s", err)
	}
	return issues, returnErr
}

func (c Checker) debugf(format string, s ...interface{}) {
	if c.Debug != nil {
		fmt.Fprint(c.Debug, "DEBUG: ")
		fmt.Fprintf(c.Debug, format+"\n", s...)
	}
}

type pos struct {
	lineNo  int // line number
	hunkPos int // position relative to first @@ in file
}

// linesChanges returns a map of file names to line numbers being changed.
// If key is nil, the file has been recently added, else it contains a slice
// of positions that have been added.
func (c Checker) linesChanged() map[string][]pos {
	type state struct {
		file    string
		lineNo  int   // current line number within chunk
		hunkPos int   // current line count since first @@ in file
		changes []pos // position of changes
	}

	var (
		s       state
		changes = make(map[string][]pos)
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
		c.debugf(line)
		s.lineNo++
		s.hunkPos++
		switch {
		case strings.HasPrefix(line, "+++ ") && len(line) > 4:
			if s.changes != nil {
				// record the last state
				changes[s.file] = s.changes
			}
			// 6 removes "+++ b/"
			s = state{file: line[6:], hunkPos: -1, changes: []pos{}}
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
			s.lineNo = int(cstart) - 1 // -1 as cstart is the next line number
		case strings.HasPrefix(line, "-"):
			s.lineNo--
		case strings.HasPrefix(line, "+"):
			s.changes = append(s.changes, pos{lineNo: s.lineNo, hunkPos: s.hunkPos})
		}

	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	// record the last state
	changes[s.file] = s.changes

	return changes
}

// readGitDiffStderr returns the error from git diff stderr.
func readGitDiffStderr(buff bytes.Buffer) error {
	output, err := io.ReadAll(&buff)
	if err != nil {
		return fmt.Errorf("could not read git diff stderr: %v", err)
	}
	return errors.New(string(output))
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
	var errBuff bytes.Buffer

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
		cmd.Stderr = &errBuff
		if err := cmd.Run(); err != nil {
			gitDiffStderr := readGitDiffStderr(errBuff)
			return nil, nil, fmt.Errorf("error executing git diff %q %q: %s\n%v", revisionFrom, revisionTo, err, gitDiffStderr)
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
	cmd.Stderr = &errBuff
	if err := cmd.Run(); err != nil {
		gitDiffStderr := readGitDiffStderr(errBuff)
		return nil, nil, fmt.Errorf("error executing git diff: %s\n%v", err, gitDiffStderr)
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
	cmd.Stderr = &errBuff
	if err := cmd.Run(); err != nil {
		gitDiffStderr := readGitDiffStderr(errBuff)
		return nil, nil, fmt.Errorf("error executing git diff HEAD~: %s\n%v", err, gitDiffStderr)
	}

	return &patch, nil, nil
}
