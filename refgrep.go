package refgrep

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type state struct {
	file    string
	cstart  uint64   // chunkstart == 2 given: @@ -1 +2,4 @@
	lineNo  uint64   // current line number within chunk
	changes []uint64 // line numbers being changes
}

func Changes() {
	fmt.Println("Checking for changes...")

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
		line := scanner.Text()
		s.lineNo++
		switch {
		case len(line) >= 4 && line[:3] == "+++":
			if s.changes != nil {
				// record the last state
				changes[s.file] = s.changes
			}
			s = state{file: line[4:]}
			fmt.Println("line:", line)
			fmt.Println("file:", s.file)
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
			fmt.Println("chdr:", line)
			fmt.Println("cstart:", s.cstart)
		case len(line) > 0 && line[:1] == "+":
			s.changes = append(s.changes, s.lineNo)
			fmt.Println(scanner.Text())
		}

	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	// record the last state
	changes[s.file] = s.changes

	fmt.Printf("changed lines: %+v\n", changes)

}
