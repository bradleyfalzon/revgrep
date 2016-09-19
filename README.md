# Overview

`refgrep` is a CLI tool used to filter static analysis tools to only lines changed based on a commit reference.

# Usage

In the scenario below, a change was made causing a warning in `go vet` on line 5, but `go vet` will show all warnings.
Using `regrep`, you can show only warnings for lines of code that have been changed (in this case, hiding line 6).

```bash
[user@host dir (master)]$ go vet
main.go:5: missing argument for Sprintf("%s"): format reads arg 1, have only 0 args
main.go:6: missing argument for Sprintf("%s"): format reads arg 1, have only 0 args
[user@host dir (master)]$ go vet | refgrep
main.go:5: missing argument for Sprintf("%s"): format reads arg 1, have only 0 args
```
