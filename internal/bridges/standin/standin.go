// Package standin gives a stand-in bridge: a bridge folder written for the
// tests and the demo, whose machines answer as they are told to. Its in
// machine waits on a ticket, and ends with what it is answered, fails with
// the words it is given as fail, or hands over the things it is given as
// hand and waits again, each { "$make": text } in them as a blob of the
// text; its out machine ends or fails alike. It reaches no service and
// needs no key.
package standin

import (
	"fmt"
	"maps"
	"strings"
)

// The tickets the stand-in machines wait on.
const (
	Heard   = "heard"
	Carried = "carried"
)

// The stand-in's script: it hands over the things it is given, with each
// { "$make": text } in them made a blob of the text, or fails with the
// words it is given.
const script = `const made = v =>
  Array.isArray(v) ? v.map(made)
  : v === null || typeof v !== "object" ? v
  : typeof v.$make === "string" ? blobs.make(v.$make)
  : Object.fromEntries(Object.entries(v).map(([k, x]) => [k, made(x)]))
if (input.hand !== undefined) {
  emit("heard", made(input.hand))
  return null
}
throw new Error(input.fail)
`

// in is the stand-in in machine, with the input it takes, cursor or
// delivery, as %s.
const in = `# A stand-in bridge's in machine: it waits for what it heard, and ends
# with it, fails with the words given as fail, or hands over the things
# given as hand and waits again.
statechart: standin-in
input: { settings: any, %s: any }
initial: waiting
states:
  waiting:
    ticket: heard
    on:
      heard:
        - { go: failing, when: "${event.data.fail !== undefined}" }
        - { go: handing, when: "${event.data.hand !== undefined}" }
        - { go: ended }
  handing:
    job: { script: ./standin.js, input: { hand: "${event.data.hand}" } }
    on: { done: waiting }
  ended: { end: done, output: "${event.data}" }
  failing:
    job: { script: ./standin.js, input: { fail: "${event.data.fail}" } }
`

// out is the stand-in out machine, with its inputs as %s.
const out = `# A stand-in bridge's out machine: it waits to be told how its carrying
# went, { sent } or { refused }, and ends with it, or fails with the words
# given as fail.
statechart: standin-out
input: { settings: any, to: string, %stext: blob }
initial: waiting
states:
  waiting:
    ticket: carried
    on:
      carried:
        - { go: failing, when: "${event.data.fail !== undefined}" }
        - { go: ended }
  ended: { end: done, output: "${event.data}" }
  failing:
    job: { script: ./standin.js, input: { fail: "${event.data.fail}" } }
`

// Folder gives a stand-in bridge's files, by their names in its folder: an
// in machine that hears when hears says so and listens when not, and an
// out machine that says things to a person directly when direct says so,
// as Telegram's does, and not when not, as GitLab's.
func Folder(hears, direct bool) map[string]string {
	input := "cursor"
	if hears {
		input = "delivery"
	}
	inputs := ""
	if direct {
		inputs = "direct: boolean, "
	}
	return map[string]string{
		"in.yaml":    fmt.Sprintf(in, input),
		"out.yaml":   fmt.Sprintf(out, inputs),
		"standin.js": script,
	}
}

// Marking gives a stand-in bridge's files with an out machine that takes
// working, as Slack's does, so it is told what a team is doing on a
// request asked at a place.
func Marking(folder map[string]string) map[string]string {
	out := maps.Clone(folder)
	out["out.yaml"] = strings.Replace(folder["out.yaml"], "text: blob }", `text: blob, working: "string?" }`, 1)
	return out
}

// In gives the files of a bridge folder, by their paths in the office,
// under bridges/<name>.
func In(name string, folder map[string]string) map[string]string {
	files := map[string]string{}
	for file, text := range folder {
		files["bridges/"+name+"/"+file] = text
	}
	return files
}

// Instead gives an office's files with a stand-in in place of a bridge
// that runs the folder Bulldog ships: its entry in office.yaml no longer
// names that folder, and its own folder, under bridges/<name>, is the
// stand-in's.
func Instead(files map[string]string, name string, folder map[string]string) map[string]string {
	out := maps.Clone(files)
	out["office.yaml"] = Unshipped(files["office.yaml"], name)
	maps.Copy(out, In(name, folder))
	return out
}

// Unshipped gives office.yaml with a bridge's entry no longer naming the
// folder Bulldog ships, on a line of its own as the examples write it, so
// that the bridge runs its own folder.
func Unshipped(office, name string) string {
	from := "    from: ./.bulldog/bridges/" + name + "\n"
	if !strings.Contains(office, from) {
		panic("office.yaml does not run the bridge " + name + " from the folder Bulldog ships")
	}
	return strings.Replace(office, from, "", 1)
}
