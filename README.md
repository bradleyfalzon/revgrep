# Overview

[![Build Status](https://travis-ci.org/bradleyfalzon/revgrep.svg?branch=master)](https://travis-ci.org/bradleyfalzon/revgrep) [![Coverage
Status](https://coveralls.io/repos/github/bradleyfalzon/revgrep/badge.svg?branch=master)](https://coveralls.io/github/bradleyfalzon/revgrep?branch=master) [GoDoc](https://godoc.org/github.com/bradleyfalzon/revgrep?status.svg)](https://godoc.org/github.com/bradleyfalzon/revgrep)

`revgrep` is a CLI tool used to filter static analysis tools to only lines changed based on a commit reference.

# Install

```bash
go get -u github.com/bradleyfalzon/revgrep/...
```

# Usage

In the scenario below, a change was made causing a warning in `go vet` on line 5, but `go vet` will show all warnings.
Using `revgrep`, you can show only warnings for lines of code that have been changed (in this case, hiding line 6).

```bash
[user@host dir (master)]$ go vet
main.go:5: missing argument for Sprintf("%s"): format reads arg 1, have only 0 args
main.go:6: missing argument for Sprintf("%s"): format reads arg 1, have only 0 args
[user@host dir (master)]$ go vet |& revgrep
main.go:5: missing argument for Sprintf("%s"): format reads arg 1, have only 0 args
```

`|&` is shown above as many static analysis programs write to `stderr`, not `stdout`, `|&` combines both `stderr` and
`stdout`. It could also be achieved with `go vet 2>&1 | revgrep`.

`revgrep` CLI tool will return an exit status of 1 if any issues match, else it will return 0. Consider using
`${PIPESTATUS[0]}` for the exit status of the `go vet` command in the above example.
