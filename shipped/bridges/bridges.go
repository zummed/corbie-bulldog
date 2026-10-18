// Package bridges holds the bridges Bulldog ships, each a folder of plain
// Corbie, an in machine, an out machine and the script they share, with
// the settings it is added to an office with. A person adds one by a
// change of their own, which copies its folder into bridges/<name>; from
// then on the folder is the office's own.
package bridges

import (
	"embed"
	"io/fs"
	"slices"
)

//go:embed telegram slack gitlab jira
var folders embed.FS

// A Shipped is a bridge Bulldog ships: its name, what it carries, and the
// settings it is added with, which Bulldog reads only the places of.
type Shipped struct {
	Name     string
	Carries  string
	Settings string
}

// All are the bridges Bulldog ships, in the order they are shown.
var All = []Shipped{
	{"telegram", "Telegram: it listens for what is said to the bot, and carries questions and decisions to a person directly, with a button for each choice.",
		"{ places: { direct: {} } }"},
	{"slack", "Slack: Slack calls the office with what is said and each press of a button; it answers in the thread, and carries questions and decisions to a person directly.",
		"{ places: { direct: {} } }"},
	{"gitlab", "GitLab issues and merge requests: it listens on the projects its places name, answers in the issue or merge request, and hears it closed.",
		"{ places: {} }"},
	{"jira", "Jira issues: it listens on the projects its places name, answers in the issue, and hears its status change.",
		"{ email: you@example.com, places: {} }"},
}

// Bridge gives a bridge Bulldog ships, by its name: the files of its
// folder, and the settings it is added with.
func Bridge(name string) (fs.FS, string, bool) {
	i := slices.IndexFunc(All, func(s Shipped) bool { return s.Name == name })
	if i < 0 {
		return nil, "", false
	}
	tree, err := fs.Sub(folders, name)
	if err != nil {
		panic(err)
	}
	return tree, All[i].Settings, true
}
