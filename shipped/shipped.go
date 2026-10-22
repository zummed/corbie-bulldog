// Package shipped is what Bulldog ships: Corbie definitions and scripts an
// office names under ./.bulldog/, from any directory of the office. Each
// shipped directory has a README.md, its page.
package shipped

import (
	"embed"
	"io/fs"
)

// Files holds what Bulldog ships, by its path under ./.bulldog/, such as
// wait/wait.yaml.
//
//go:embed wait clock docs tools rules bridges
var Files embed.FS

// Read gives a file Bulldog ships, by its path under ./.bulldog/.
func Read(path string) ([]byte, error) {
	return fs.ReadFile(Files, path)
}
