package revgrep

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestChanges(t *testing.T) {

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working dir: %s", err)
	}

	// Execute make
	cmd := exec.Command("./make.sh")
	cmd.Dir = filepath.Join(cwd, "testdata")
	err = cmd.Run()
	if err != nil {
		t.Fatalf("could not run make.sh: %v", err)
	}

	// chdir so the vcs exec commands read the correct testdata
	err = os.Chdir(filepath.Join(cwd, "testdata", "git"))
	if err != nil {
		t.Fatalf("could not chdir: %v", err)
	}

	writer := bytes.NewBuffer([]byte{})
	reader := bytes.NewBufferString(`main.go:5: missing argument
main.go:6: missing argument
`)
	exp := "main.go:5: missing argument\n"

	// the vcs (thanks to make.sh) will alert us line 5 has changed
	// reader shows multiple lines are affected
	// writer should just have the lines changes according to vcs
	// exp is what's expected
	Changes(reader, writer)

	if writer.String() != exp {
		t.Errorf("exp:\n%q\ngot:\n%q\n", exp, writer.String())
	}

}
