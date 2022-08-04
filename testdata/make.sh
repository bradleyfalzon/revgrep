#!/usr/bin/env bash
set -u

if [[ "$(basename $(pwd))" != "testdata" ]]; then
    echo Run for testdata dir
    exit 1
fi

[[ -d git ]] && rm -rf git
mkdir -p git/subdir && cd git

# No git directory

[[ "$1" == "1-non-git-dir" ]] && exit

# Untracked files

git init --initial-branch=main
git config --local user.name "testdata"
git config --local user.email "testdata@example.com"
git config --local color.diff always

if [[ "$(go env GOOS)" == "windows" ]]; then
    git config --local core.autocrlf false
fi

touch readme
git add .
git commit -m "Initial commit"

cat > main.go <<'EOF'
package main
import "fmt"
var _ = fmt.Sprintf("2-untracked %s")
func main() {}
EOF

[[ "$1" == "2-untracked" ]] && exit

# Untracked files with sub dir

cat > subdir/main.go <<'EOF'
package main
import "fmt"
var _ = fmt.Sprintf("3-untracked-subdir %s")
func main() {}
EOF

[[ "$1" == "3-untracked-subdir" ]] && exit

# Placeholder for test to change to sub directory

[[ "$1" == "3-untracked-subdir-cwd" ]] && exit

# Commit

git add .
git commit -m "Commit"

[[ "$1" == "4-commit" ]] && exit

# Unstage changes without warning

cat >> main.go <<'EOF'
var _ = fmt.Sprintln("5-unstaged-no-warning")
EOF

[[ "$1" == "5-unstaged-no-warning" ]] && exit

cat >> main.go <<'EOF'
var _ = fmt.Sprintf("6-unstaged %s")
EOF

[[ "$1" == "6-unstaged" ]] && exit

# Commit all changes

git add .
git commit -m "Commit"

[[ "$1" == "7-commit" ]] && exit

cat >> main.go <<'EOF'
var _ = fmt.Sprintf("8-unstaged %s")
EOF

[[ "$1" == "8-unstaged" ]] && exit

cat > main2.go <<'EOF'
package main
import "fmt"
var _ = fmt.Sprintf("9-untracked %s")
EOF

[[ "$1" == "9-untracked" ]] && exit

# Placeholder for test to check committed changes

[[ "$1" == "10-committed" ]] && exit

# Display absolute path

[[ "$1" == "11-abs-path" ]] && exit

# Remove one line on a file with existing issues

if [[ "$1" == "12-removed-lines" ]]; then

    cat > 12-removed-lines.go <<'EOF'
package main
import "fmt"
var _ = fmt.Sprintf("12-removed-lines %s")
// some comment that will be removed
EOF

    git add .
    git commit -m "Commit"

    cat > 12-removed-lines.go <<'EOF'
package main
import "fmt"
var _ = fmt.Sprintf("12-removed-lines %s")
EOF

    git add .
    git commit -m "Commit"
fi
