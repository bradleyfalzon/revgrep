#!/usr/bin/env bash
set -eu

if [[ "$(basename $(pwd))" != "testdata" ]]; then
    echo Run for testdata dir
    exit 1
fi

[[ -d git ]] && rm -rf git

mkdir git && cd git
git init
git config --local user.name "testdata"
git config --local user.email "testdata@example.com"

# Initial commit
cat > main.go <<EOF
package main
EOF

git add .
git commit -m "Initial commit"

# Unstage changes with go vet warning
cat >> main.go <<EOF
import "fmt"
var _ = fmt.Sprintf("%s") // main.go:3: missing argument for Sprintf("%s")...
func main() {}
EOF
