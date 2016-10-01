#!/usr/bin/env bash
set -u

if [[ "$(basename $(pwd))" != "testdata" ]]; then
    echo Run for testdata dir
    exit 1
fi

function close() {
    go vet ./...
    exit 0
}

[[ -d git ]] && rm -rf git
mkdir -p git/subdir && cd git

# No git directory

[[ "$1" == "1-non-git-dir" ]] && exit

# Untracked files

git init > /dev/null
git config --local user.name "testdata"
git config --local user.email "testdata@example.com"
touch readme
git add .
git commit -m "Initial commit" > /dev/null

cat > main.go <<EOF
package main
import "fmt"
var _ = fmt.Sprintf("2-untracked %s")
func main() {}
EOF

[[ "$1" == "2-untracked" ]] && close

# Untracked files with sub dir

cat > subdir/main.go <<EOF
package main
import "fmt"
var _ = fmt.Sprintf("3-untracked-subdir %s")
func main() {}
EOF

[[ "$1" == "3-untracked-subdir" ]] && close

# Placeholder for test to change to sub directory

[[ "$1" == "3-untracked-subdir-cwd" ]] && close

# Commit

git add .
git commit -m "Commit" > /dev/null

[[ "$1" == "4-commit" ]] && close

# Unstage changes without warning

cat >> main.go <<EOF
var _ = fmt.Sprintln("5-unstaged-no-warning")
EOF

[[ "$1" == "5-unstaged-no-warning" ]] && close

cat >> main.go <<EOF
var _ = fmt.Sprintf("6-unstaged %s")
EOF

[[ "$1" == "6-unstaged" ]] && close

# Commit all changes

git add .
git commit -m "Commit" > /dev/null

[[ "$1" == "7-commit" ]] && close

cat >> main.go <<EOF
var _ = fmt.Sprintf("8-unstaged %s")
EOF

[[ "$1" == "8-unstaged" ]] && close

cat > main2.go <<EOF
package main
var _ = fmt.Sprintf("9-untracked %s")
EOF

[[ "$1" == "9-untracked" ]] && close

# Placeholder for test to check committed changes

[[ "$1" == "10-committed" ]] && close
