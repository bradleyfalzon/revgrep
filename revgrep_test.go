package revgrep

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func setup(t *testing.T, stage, subdir string) (prevwd string, sample []byte) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working dir: %s", err)
	}

	// Execute make
	cmd := exec.Command("./make.sh", stage)
	cmd.Dir = filepath.Join(wd, "testdata")
	sample, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("could not run make.sh: %v, output:\n%s", err, sample)
	}

	// chdir so the vcs exec commands read the correct testdata
	err = os.Chdir(filepath.Join(wd, "testdata", "git", subdir))
	if err != nil {
		t.Fatalf("could not chdir: %v", err)
	}
	return wd, sample
}

func teardown(t *testing.T, wd string) {
	err := os.Chdir(wd)
	if err != nil {
		t.Fatalf("could not chdir: %v", err)
	}
}

// TestCheckerRegexp tests line matching and extraction of issue
func TestCheckerRegexp(t *testing.T) {
	tests := []struct {
		regexp string
		line   string
		want   Issue
	}{
		{"", "file.go:1:issue", Issue{"file.go", 1, 0, 2, "file.go:1:issue", "issue"}},
		{"", "file.go:1:5:issue", Issue{"file.go", 1, 5, 2, "file.go:1:5:issue", "issue"}},
		{"", "file.go:1:  issue", Issue{"file.go", 1, 0, 2, "file.go:1:  issue", "issue"}},
		{`.*?:(.*?\.go):([0-9]+):()(.*)`, "prefix:file.go:1:issue", Issue{"file.go", 1, 0, 2, "prefix:file.go:1:issue", "issue"}},
	}

	diff := []byte(`--- a/file.go
+++ b/file.go
@@ -1,1 +1,1 @@
-func Line() {}
+func NewLine() {}`)

	for _, test := range tests {
		checker := Checker{
			Patch:  bytes.NewReader(diff),
			Regexp: test.regexp,
		}

		issues, err := checker.Check(bytes.NewReader([]byte(test.line)), ioutil.Discard)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		want := []Issue{test.want}
		if !reflect.DeepEqual(issues, want) {
			t.Errorf("unexpected issues for line: %q\nhave: %#v\nwant: %#v", test.line, issues, want)
		}
	}
}

// TestChangesReturn tests the writer in the argument to the Changes function
// and generally tests the entire programs functionality.
func TestChangesWriter(t *testing.T) {
	tests := map[string]struct {
		subdir  string
		exp     []string // file:linenumber including trailing colon
		revFrom string
		revTo   string
	}{
		"2-untracked":            {"", []string{"main.go:3:"}, "", ""},
		"3-untracked-subdir":     {"", []string{"main.go:3:", "subdir/main.go:3:"}, "", ""},
		"3-untracked-subdir-cwd": {"subdir", []string{"main.go:3:"}, "", ""},
		"4-commit":               {"", []string{"main.go:3:", "subdir/main.go:3:"}, "", ""},
		"5-unstaged-no-warning":  {"", nil, "", ""},
		"6-unstaged":             {"", []string{"main.go:6:"}, "", ""},
		// From a commit, all changes should be shown
		"7-commit": {"", []string{"main.go:6:"}, "HEAD~1", ""},
		// From a commit+unstaged, all changes should be shown
		"8-unstaged": {"", []string{"main.go:6:", "main.go:7:"}, "HEAD~1", ""},
		// From a commit+unstaged+untracked, all changes should be shown
		"9-untracked": {"", []string{"main.go:6:", "main.go:7:", "main2.go:3:"}, "HEAD~1", ""},
		// From a commit to last commit, all changes should be shown except recent unstaged, untracked
		"10-committed": {"", []string{"main.go:6:"}, "HEAD~1", "HEAD~0"},
		// static analysis tools with absolute paths should be handled
		"11-abs-path": {"", []string{"main.go:6:"}, "HEAD~1", "HEAD~0"},
		// Removing a single line shouldn't raise any issues.
		"12-removed-lines": {"", nil, "", ""},
	}

	for stage, test := range tests {
		t.Run(stage, func(t *testing.T) {
			prevwd, sample := setup(t, stage, test.subdir)
			reader := bytes.NewBuffer(sample)
			var out bytes.Buffer

			c := Checker{
				RevisionFrom: test.revFrom,
				RevisionTo:   test.revTo,
			}
			_, err := c.Check(reader, &out)
			if err != nil {
				t.Errorf("%v: unexpected error: %v", stage, err)
			}
			scanner := bufio.NewScanner(&out)
			var i int
			for i = 0; scanner.Scan(); i++ {
				// Rewrite abs paths to for simpler matching
				line := rewriteAbs(scanner.Text())
				line = strings.TrimPrefix(line, "./")

				if i > len(test.exp)-1 {
					t.Errorf("%v: unexpected line: %q", stage, line)
				} else {
					if !strings.HasPrefix(line, test.exp[i]) {
						t.Errorf("%v: line does not have prefix: %q line: %q", stage, test.exp[i], line)
					}
				}
			}
			if i != len(test.exp) {
				t.Errorf("%v: i %v, expected %v", stage, i, len(test.exp))
			}
			teardown(t, prevwd)
		})
	}
}

func rewriteAbs(line string) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return strings.TrimPrefix(line, cwd+"/")
}

func TestGitPatchNonGitDir(t *testing.T) {
	// Change to non-git dir
	err := os.Chdir("/")
	if err != nil {
		t.Fatalf("could not chdir: %v", err)
	}

	patch, newfiles, err := GitPatch("", "")
	if err != nil {
		t.Errorf("error expected nil, got: %v", err)
	}
	if patch != nil {
		t.Errorf("patch expected nil, got: %v", patch)
	}
	if newfiles != nil {
		t.Errorf("newFiles expected nil, got: %v", newfiles)
	}
}

func TestLinesChanged(t *testing.T) {
	diff := []byte(`--- a/file.go
+++ b/file.go
@@ -1,1 +1,1 @@
 // comment
-func Line() {}
+func NewLine() {}
@@ -20,1 +20,1 @@
 // comment
-func Line() {}
+func NewLine() {}
 // comment
@@ -3,1 +30,1 @@
-func Line() {}
+func NewLine() {}
 // comment`)

	checker := Checker{
		Patch: bytes.NewReader(diff),
	}

	have := checker.linesChanged()

	want := map[string][]pos{
		"file.go": []pos{
			{lineNo: 2, hunkPos: 3},
			{lineNo: 21, hunkPos: 7},
			{lineNo: 30, hunkPos: 11},
		},
	}

	if !reflect.DeepEqual(have, want) {
		t.Errorf("unexpected pos:\nhave: %#v\nwant: %#v", have, want)
	}
}
