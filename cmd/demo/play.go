package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// socket is what Kit asks the house when the demo starts.
const socket = "The socket by the back door sparks when the mower is plugged in."

// stock is what a worker says to anything the demo has no part for.
const stock = "This is the demo, and I am a stand-in for a model. I play one morning of the house: a socket that sparks."

// play is the stand-in's part: what each worker of the house says, known
// by its system text, turn by turn. The steward asks the garden about the
// mower, then hands the socket to the fixer. The fixer asks sparks which
// circuit the socket is on, cuts its power, which waits on Sam, and orders
// a socket for £12, which small-orders.js lets through.
func play(turn testkit.Turn) testkit.Reply {
	switch {
	case strings.Contains(turn.System, "You look after the house"):
		return steward(turn)
	case strings.Contains(turn.System, "You mend things"):
		return fixer(turn)
	case strings.Contains(turn.System, "You know the house's wiring"):
		return testkit.Say("The socket by the back door is on circuit 4, the kitchen sockets.")
	case strings.Contains(turn.System, "You keep the house's accounts"):
		return testkit.Say(`{"says": "pass", "note": "It costs nothing, so Sam decides."}`)
	case strings.Contains(turn.System, "You look after the garden") && strings.Contains(turn.Prompt(), "mower"):
		return testkit.Say("The mower's lead is frayed at the plug. The mower stays in the shed until it has a new lead.")
	case strings.Contains(turn.System, "You look after the garden"):
		return testkit.Say("The lawn wants mowing on Saturday, once the mower has a new lead.")
	}
	return testkit.Say(stock)
}

// steward plays the house's lead.
func steward(turn testkit.Turn) testkit.Reply {
	switch {
	case strings.Contains(turn.Prompt(), "still open"):
		for _, line := range strings.Split(turn.System, "\n") {
			if strings.HasPrefix(line, "Requests of ") {
				return testkit.Say(line)
			}
		}
	case !strings.Contains(turn.Prompt(), "sparks"):
	case turn.Number == 1:
		return testkit.Use("ask_team", map[string]any{"team": "garden", "question": "The socket by the back door sparked with the mower plugged in. Is the mower all right?"})
	case turn.Number == 2:
		return testkit.Use("fixer", map[string]any{"repair": turn.Prompt()})
	case strings.Contains(turn.Last(), "Circuit 4 is off"):
		return testkit.Say("The fixer cut circuit 4 and ordered a new socket for £12, which it fits when it comes. The garden says the mower's lead is frayed, so the mower stays in the shed until it has a new one.")
	default:
		return testkit.Say("The socket is not mended, so keep away from it. The fixer says: " + turn.Last())
	}
	return testkit.Say(stock)
}

// fixer plays the steward's colleague.
func fixer(turn testkit.Turn) testkit.Reply {
	switch {
	case turn.Number == 1:
		return testkit.Use("sparks", map[string]any{"question": "Which circuit is the socket by the back door on?"})
	case turn.Number == 2:
		return testkit.Use("cut_power", map[string]any{"circuit": 4})
	case turn.Number == 3 && strings.Contains(turn.Last(), "Circuit 4 is off"):
		return testkit.Use("order_part", map[string]any{"part": "a 13A socket", "pounds": 12})
	case turn.Number == 3:
		return testkit.Say("Sam did not agree to cut circuit 4, so I left the socket as it is.")
	case strings.Contains(turn.Last(), "Ordered"):
		return testkit.Say("Circuit 4 is off, and a 13A socket is ordered for £12. I fit it when it comes.")
	}
	return testkit.Say("Circuit 4 is off, but no socket is ordered.")
}

// photo gives Kit's photo of the socket, as a PNG: a faceplate, its two
// holes, and a scorch mark between them.
func photo() []byte {
	img := image.NewGray(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			shade := uint8(235)
			switch {
			case (x == 10 || x == 21) && y >= 12 && y < 20:
				shade = 40
			case x > 10 && x < 21 && y >= 14 && y < 18:
				shade = 120
			}
			img.SetGray(x, y, color.Gray{Y: shade})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}
