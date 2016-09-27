package revgrep

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setup(t *testing.T) (prevwd string) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working dir: %s", err)
	}

	// Execute make
	cmd := exec.Command("./make.sh")
	cmd.Dir = filepath.Join(wd, "testdata")
	err = cmd.Run()
	if err != nil {
		t.Fatalf("could not run make.sh: %v", err)
	}

	// chdir so the vcs exec commands read the correct testdata
	err = os.Chdir(filepath.Join(wd, "testdata", "git"))
	if err != nil {
		t.Fatalf("could not chdir: %v", err)
	}
	return wd
}

func teardown(t *testing.T, wd string) {
	err := os.Chdir(wd)
	if err != nil {
		t.Fatalf("could not chdir: %v", err)
	}
}

// TestChanges is a complete test using testdata
func TestChanges(t *testing.T) {
	prevwd := setup(t)
	defer teardown(t, prevwd)

	writer := bytes.NewBuffer([]byte{})
	reader := bytes.NewBufferString(`main.go:3: missing argument
main.go:9999: missing argument
`)
	exp := "main.go:3: missing argument\n"

	// the vcs (thanks to make.sh) will alert us line 3 has changed
	// reader shows multiple lines are affected
	// writer should just have the lines changes according to vcs
	// exp is what's expected
	checker := Checker{}
	checker.Check(reader, writer)

	if writer.String() != exp {
		t.Errorf("exp:\n%q\ngot:\n%q\n", exp, writer.String())
	}

}

func TestGitPatch(t *testing.T) {
	prevwd := setup(t)
	defer teardown(t, prevwd)

	exp := []byte(`+var _ = fmt.Sprintf("%s") // main.go:3: missing argument for Sprintf("%s")...`)

	// Test for unstaged changes

	patch, err := GitPatch()
	if err != nil {
		t.Fatalf("unexpected error from git: %v", err)
	}
	if !bytes.Contains(patch.(*bytes.Buffer).Bytes(), exp) {
		t.Fatalf("GitPatch did not detect unstaged changes")
	}

	// Commit

	err = exec.Command("git", "add", ".").Run()
	if err != nil {
		t.Fatalf("could not commit changes: %v", err)
	}

	err = exec.Command("git", "commit", "-am", "TestGitPatch").Run()
	if err != nil {
		t.Fatalf("could not commit changes: %v", err)
	}

	// Test for last commit

	patch, err = GitPatch()
	if err != nil {
		t.Fatalf("unexpected error from git: %v", err)
	}
	if !bytes.Contains(patch.(*bytes.Buffer).Bytes(), exp) {
		t.Fatalf("GitPatch did not detect committed changes\n%s\ndoes not contain: %s",
			patch.(*bytes.Buffer).Bytes(), exp,
		)
	}

	// Change to non-git dir

	err = os.Chdir("/")
	if err != nil {
		t.Fatalf("could not chdir: %v", err)
	}

	// Test for handling non git dir

	patch, err = GitPatch()
	if err != nil {
		t.Fatalf("unexpected error from git: %v", err)
	}
	if patch != nil {
		t.Fatalf("expected nil, got %v", patch)
	}

}

func TestGitPatchUntracked(t *testing.T) {
	prevwd := setup(t)
	defer teardown(t, prevwd)

	exp := []byte(`+var _ = fmt.Sprintf("untracked %v") // untracked.go:2: missing argument for Sprintf("untracked %v")...`)

	// Test for unstaged changes

	patch, err := GitPatch()
	if err != nil {
		t.Fatalf("unexpected error from git: %v", err)
	}
	if !bytes.Contains(patch.(*bytes.Buffer).Bytes(), exp) {
		t.Fatalf("GitPatch did not detect untracked changes\n%s\ndoes not contain: %s",
			patch.(*bytes.Buffer).Bytes(), exp,
		)
	}
}
