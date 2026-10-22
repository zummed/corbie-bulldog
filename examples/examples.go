// Package examples holds two offices as a person writes them: the house,
// and the work office.
package examples

import (
	"embed"
	"io/fs"
)

//go:embed house work
var offices embed.FS

// House is the house: Sam and Kit, the steward and its colleagues, and the
// garden within.
var House = sub("house")

// Work is the work office: one Slack bot over payments, platform and
// support.
var Work = sub("work")

func sub(dir string) fs.FS {
	tree, err := fs.Sub(offices, dir)
	if err != nil {
		panic(err)
	}
	return tree
}
