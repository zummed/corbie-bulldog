// Package standin gives a stand-in bridge: a bridge folder written for the
// tests and the demo, whose machines answer as they are told to. Its in
// machine waits on a ticket, and ends with what it is answered, or fails
// with the words it is given as fail; so does its out machine. It reaches
// no service and needs no key.
package standin

import "fmt"

// The tickets the stand-in machines wait on.
const (
	Heard   = "heard"
	Carried = "carried"
)

// The stand-in's script: it fails with the words it is given.
const script = `throw new Error(input.fail)
`

// in is the stand-in in machine, with the input it takes, cursor or
// delivery, as %s.
const in = `# A stand-in bridge's in machine: it waits for what it heard, and ends
# with it, or fails with the words given as fail.
statechart: standin-in
input: [settings, %s]
initial: waiting
states:
  waiting:
    ticket: heard
    on:
      heard:
        - { go: failing, when: "${event.data.fail !== undefined}" }
        - { go: ended }
  ended: { end: done, output: "${event.data}" }
  failing:
    job: { script: ./standin.js, input: { fail: "${event.data.fail}" } }
`

// out is the stand-in out machine, with its inputs as %s.
const out = `# A stand-in bridge's out machine: it waits to be told how its carrying
# went, { sent } or { refused }, and ends with it, or fails with the words
# given as fail.
statechart: standin-out
input: [settings, to, %stext]
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
		inputs = "direct, "
	}
	return map[string]string{
		"in.yaml":    fmt.Sprintf(in, input),
		"out.yaml":   fmt.Sprintf(out, inputs),
		"standin.js": script,
	}
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
