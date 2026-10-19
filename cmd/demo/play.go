package main

import (
	"slices"
	"strings"
	"unicode"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// Stock is what the steward says to anything the demo has no part for.
const Stock = "This is the demo, and I am its stand-in for a model, so I play only the house's day: " +
	"say a socket sparked or the bathroom fan has stopped, ask after the lawn, or ask what was spent."

// play is the stand-in's part: what each worker of the house says, known
// by its system text and what it was asked, turn by turn. The steward
// hands a socket that sparked and a fan that stopped to the fixer, the
// garden to the garden, and money to the clerk, and passes on what it is
// told; on its walk round it says what is open. The fixer asks sparks
// which circuit, cuts it, orders a socket, asks to be told when it comes,
// and then fits it; for the fan it orders one that costs more than the
// house orders alone, and says whether it was ordered. The gardener hands
// the heavy work to the groundsman.
func play(turn testkit.Turn) testkit.Reply {
	asked := said(turn)
	switch {
	case strings.Contains(turn.System, "You look after the house"):
		return steward(turn, asked)
	case strings.Contains(turn.System, "You do the repairs"):
		return fixer(turn, asked)
	case strings.Contains(turn.System, "You know the house's wiring"):
		if turn.Number == 1 {
			return testkit.Use("circuits", map[string]any{})
		}
		return testkit.Say("4 kitchen ring: sockets by the window and the hob.")
	case strings.Contains(turn.System, "You keep the house's accounts"):
		return testkit.Say("This month the house spent £12, on a 13 A double socket, which is on its way. Nothing is owed.")
	case strings.Contains(turn.System, "You look after the garden"):
		switch {
		case has(asked, "weekend"):
			return testkit.Say("The lawn wants mowing on Saturday, if it has dried.")
		case turn.Number == 1:
			return testkit.Use("groundsman", map[string]any{"job": asked})
		}
		return testkit.Say("The groundsman has been out. " + turn.Last())
	case strings.Contains(turn.System, "You dig, mow and keep the shed"):
		return testkit.Say(groundsman(asked))
	}
	return testkit.Say("This is the demo's stand-in for a model, which has no part for this worker.")
}

// steward plays the house's lead.
func steward(turn testkit.Turn, asked string) testkit.Reply {
	if strings.Contains(asked, "walk round") {
		return walkRound(turn.Prompt())
	}
	is := scene(asked)
	if turn.Number > 1 {
		told := turn.Last()
		// A team's answer comes as who answered for it, then the answer;
		// anything else that comes back is passed on whole.
		if _, answer, ok := strings.Cut(told, ":\n\n"); ok && is == "garden" {
			told = answer
		}
		return testkit.Say(teller[is] + told)
	}
	switch is {
	case "socket":
		return testkit.Use("fixer", map[string]any{"job": "A socket sparked: make it safe, and mend it."})
	case "fan":
		return testkit.Use("fixer", map[string]any{"job": "The bathroom's extractor fan has stopped: replace it."})
	case "garden":
		return testkit.Use("ask_team", map[string]any{"team": "garden", "question": asked})
	case "money":
		return testkit.Use("clerk", map[string]any{"question": asked})
	}
	return testkit.Say(Stock)
}

// teller is how the steward passes on what it was told, by the part of
// the day it is.
var teller = map[string]string{
	"socket": "The fixer says: ",
	"fan":    "The fixer says: ",
	"garden": "The garden says: ",
	"money":  "The clerk says: ",
}

// fixer plays the fixer: the fan, or else the socket.
func fixer(turn testkit.Turn, asked string) testkit.Reply {
	ordered := strings.Contains(turn.Last(), "Ordered")
	if scene(asked) == "fan" {
		switch {
		case turn.Number == 1:
			return testkit.Use("order_part", map[string]any{"part": "extractor fan, 100 mm", "pounds": 48})
		case ordered:
			return testkit.Say("A new extractor fan, 100 mm, is ordered for £48. I will fit it when it comes.")
		}
		return testkit.Say("The extractor fan is not ordered, as £48 was not agreed.")
	}
	switch turn.Number {
	case 1:
		return testkit.Use("sparks", map[string]any{"question": "Which circuit is the kitchen socket on?"})
	case 2:
		return testkit.Use("cut_power", map[string]any{"circuit": "4"})
	case 3:
		if !strings.Contains(turn.Last(), "is off") {
			return testkit.Say("Circuit 4 is still on, so I have not touched the socket. Keep away from it.")
		}
		return testkit.Use("order_part", map[string]any{"part": "13 A double socket", "pounds": 12})
	case 4:
		if !ordered {
			return testkit.Say("Circuit 4 is off, so the socket is safe, but no new socket is ordered, so it stays off until one is.")
		}
		return testkit.Use("ask_person", map[string]any{"question": "Tell me when the new socket arrives, and I'll fit it."})
	}
	return testkit.Say("The new socket is fitted, and circuit 4 is back on.")
}

// groundsman says what the groundsman did.
func groundsman(job string) string {
	switch {
	case has(job, "lawn"):
		return "I walked the lawn after the rain: it is wet but sound, with no standing water. I will mow it on Saturday if it has dried."
	case has(job, "shed"):
		return "The shed is dry inside and locked, and its key is back on the hook by the back door."
	case has(job, "hedge", "hedges"):
		return "I trimmed the hedge along the lane, and took the cuttings to the heap."
	}
	return "I did the heavy work asked of me."
}

// walkRound says what is still open, from the open requests the routine's
// run is given.
func walkRound(context string) testkit.Reply {
	var open []string
	for _, line := range strings.Split(context, "\n") {
		if strings.HasPrefix(line, "- Request ") {
			open = append(open, line)
		}
	}
	if len(open) == 0 {
		return testkit.Say("Nothing is open.")
	}
	return testkit.Say("Still open, newest first:\n" + strings.Join(open, "\n"))
}

// said gives what a worker was asked, lower-cased: of a run given the
// whole context, what is new for it, each entry without its time and who
// said it, and of a colleague called as a tool, its whole prompt.
func said(turn testkit.Turn) string {
	asked := turn.Prompt()
	if _, latest, ok := strings.Cut(asked, "\nNew for you:\n"); ok {
		latest, _, _ = strings.Cut(latest, "\n\nYour last reply")
		var lines []string
		for _, line := range strings.Split(latest, "\n") {
			if _, words, ok := strings.Cut(line, "] "); ok && strings.HasPrefix(line, "[") {
				_, line, _ = strings.Cut(words, ": ")
			}
			lines = append(lines, line)
		}
		asked = strings.Join(lines, "\n")
	}
	return strings.ToLower(asked)
}

// scene gives which part of the day something asked is.
func scene(asked string) string {
	switch {
	case has(asked, "sparked", "sparks", "sparking", "fuse", "fuses", "flicker", "flickers", "flickering"):
		return "socket"
	case has(asked, "fan", "fans"):
		return "fan"
	case has(asked, "lawn", "garden", "shed", "hedge", "hedges"):
		return "garden"
	case has(asked, "spent", "spend", "owe", "owed", "cost", "costs", "bill", "bills"):
		return "money"
	}
	return ""
}

// has says whether something asked holds any of some words: whole words,
// so that the power is not money, a fancy light no fan, and what was
// washed no shed.
func has(asked string, some ...string) bool {
	words := strings.FieldsFunc(asked, func(r rune) bool { return !unicode.IsLetter(r) })
	return slices.ContainsFunc(words, func(w string) bool { return slices.Contains(some, w) })
}
