// Package examples holds the offices the pages show, as a person writes
// them: the house, with the Telegram bridge Sam added as its first change,
// and the work office, one Slack bot over many teams.
package examples

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
)

//go:embed house work
var offices embed.FS

// House is the house, with its Telegram bridge.
func House() fs.FS { return sub("house") }

// Work is the work office.
func Work() fs.FS { return sub("work") }

func sub(dir string) fs.FS {
	tree, err := fs.Sub(offices, dir)
	if err != nil {
		panic(err)
	}
	return tree
}

// telegram is who a person is on the Telegram bridge, as the house's
// office.yaml writes it beside their name and title.
var telegram = regexp.MustCompile(`, telegram: "[^"]*"`)

// NewHouse gives the files of the office written into an empty folder:
// the house without its bridge, spoken to in the built-in chat alone. It
// has no bridges key, no bridges/ folder and no Telegram identities.
func NewHouse() map[string][]byte {
	files := map[string][]byte{}
	err := fs.WalkDir(House(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(name, "bridges/") {
			return err
		}
		contents, err := fs.ReadFile(House(), name)
		files[name] = contents
		return err
	})
	if err != nil {
		panic(err)
	}
	var kept []string
	inBridges := false
	for _, line := range strings.SplitAfter(string(files["office.yaml"]), "\n") {
		top := line != "" && line[0] != ' ' && line[0] != '\n' && line[0] != '#'
		if top {
			inBridges = strings.HasPrefix(line, "bridges:")
		}
		if !inBridges {
			kept = append(kept, telegram.ReplaceAllString(line, ""))
		}
	}
	files["office.yaml"] = []byte(strings.Join(kept, ""))
	return files
}
