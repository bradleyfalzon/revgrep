package revgrep

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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

func TestChanges(t *testing.T) {
	tests := map[string]struct {
		subdir  string
		exp     []string // file:linenumber including trailing space
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
		"9-untracked": {"", []string{"main.go:6:", "main.go:7:", "main2.go:2:"}, "HEAD~1", ""},
		// From a commit to last commit, all changes should be shown except recent unstaged, untracked
		"10-committed": {"", []string{"main.go:6:"}, "HEAD~1", "HEAD~0"},
		// static analysis tools with absolute paths should be handled
		"11-abs-path": {"", []string{"main.go:6:"}, "HEAD~1", "HEAD~0"},
	}

	for stage, test := range tests {
		prevwd, sample := setup(t, stage, test.subdir)

		reader := bytes.NewBuffer(sample)
		var out bytes.Buffer

		c := Checker{
			RevisionFrom: test.revFrom,
			RevisionTo:   test.revTo,
		}
		_ = c.Check(reader, &out)

		scanner := bufio.NewScanner(&out)
		var i int
		for i = 0; scanner.Scan(); i++ {
			// Rewrite abs paths to for simpler matching
			line := rewriteAbs(scanner.Text())

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
