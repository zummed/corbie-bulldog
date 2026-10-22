package office

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/permission"

	_ "github.com/zummed/corbie/agent"
	_ "github.com/zummed/corbie/flow"
	_ "github.com/zummed/corbie/plugin/bundled"
	_ "github.com/zummed/corbie/statechart"
)

// shipped stands in for what Bulldog ships.
var shipped = fstest.MapFS{
	"wait/wait.yaml": {Data: []byte(`statechart: wait
input: { to: map, said: string, choices: "list?" }
initial: waiting
states:
  waiting:
    ticket: answer
    timers: { too_long: 7d }
    on: { answer: answered, too_long: unanswered }
  answered: { end: done, output: "${event.data}" }
  unanswered: { end: done, output: { answer: null, note: "No answer came within 7 days." } }
`)},
	"tools/talk/talk.yaml": {Data: []byte(`ask_person:
  does: Ask a person and wait for the answer
  args: { who: string, question: string }
  job:
    machine: ./.bulldog/wait/wait.yaml
    input: { to: { people: ["${args.who}"] }, said: "${args.question}" }
`)},
	"tools/docs/docs.yaml":         {Data: []byte("read_doc:\n  does: Read a doc\n  args: { name: string }\n  job: { script: ./docs.js }\n")},
	"tools/docs/docs.js":           {Data: []byte("return ''\n")},
	"rules/mentioned/mentioned.js": {Data: []byte("return { says: 'yes', note: '' }\n")},
	"rules/for-us/for-us.yaml":     {Data: []byte("flow: for-us\nstages: {}\n")},
	"bridges/telegram/in.yaml":     {Data: []byte("statechart: telegram-in\n")},
	"bridges/telegram/out.yaml":    {Data: []byte("flow: telegram-out\n")},
	"bridges/slack/in.yaml":        {Data: []byte("statechart: slack-in\n")},
	"bridges/slack/out.yaml":       {Data: []byte("flow: slack-out\n")},
	"bridges/gitlab/in.yaml":       {Data: []byte("statechart: gitlab-in\n")},
	"bridges/gitlab/out.yaml":      {Data: []byte("flow: gitlab-out\n")},
}

// house is the house of the design: a top team and a garden within it.
func house() map[string]string {
	return map[string]string{
		"office.yaml": `name: The house
about: The house and everything in it, but the garden.
zone: Europe/London
everyone:
  sam: { name: Sam, telegram: "4471" }
  kit: { name: Kit, telegram: "4472" }
bridges:
  telegram:
    from: ./.bulldog/bridges/telegram
    places:
      direct: {}                       # Sam and Kit ask the house in a direct message
models:
  usual: { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }
people: [sam, kit]
lead: steward
decide: [./small-orders.js, clerk, sam]
routines:
  - { at: "mon 09:00", say: "What is still open?" }
`,
		"steward.yaml": `agent: steward
model: usual
system: |
  You look after the house for Sam and Kit. Hand repairs to the fixer, and ask the garden about the garden.
  ${input.context}
prompt: ${input.said}
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]
tools:
  fixer:
    does: Repairs about the house
    args: { repair: string }
    job: { machine: ./fixer.yaml, input: { said: "${args.repair}", context: "${input.context}", request: "${input.request}" } }
`,
		"fixer.yaml": `agent: fixer
model: usual
system: You mend things about the house.
prompt: ${input.said}
tools:
  order_part:
    does: Order a part
    args: { part: string, pounds: number }
    job: { script: ./tools/order.js, approve: true }
`,
		"clerk.yaml": `agent: clerk
model: usual
system: You keep the house's accounts. Say yes, no or pass, with a note.
prompt: ${input.request.said}
reply: { says: string, note: "string?" }
`,
		"small-orders.js": `const pounds = input.jobs.map(j => (j.asked?.args ?? j.input).pounds ?? Infinity)
if (pounds.every(p => p <= 20)) return { says: "yes", note: "Under £20." }
return { says: "pass", note: "More than I order alone." }
`,
		"tools/order.js": "return { ordered: true }\n",
		"garden/team.yaml": `name: The garden
about: The beds, the lawn and the shed.
people: [sam]
lead: gardener
models:
  usual: { name: claude-opus-5-5, secret: ANTHROPIC_KEY }   # the garden's own
routines:
  - { at: "fri 16:00", say: "What needs doing before the weekend?" }
`,
		"garden/gardener.yaml": `agent: gardener
model: usual
system: You look after the garden.
prompt: ${input.said}
tools_file: [./.bulldog/tools/talk/talk.yaml]
`,
	}
}

// work is the work office of the design: one Slack bot over three teams.
func work() map[string]string {
	return map[string]string{
		"office.yaml": `name: Platform support
about: "Help with the platform and with payments, asked in #help."
zone: Europe/London
everyone:
  al:    { name: Al, slack: U0AL0000, gitlab: "1041" }
  dana:  { name: Dana, slack: U0DANA00 }
  lee:   { name: Lee, slack: U0LEE000 }
  priya: { name: Priya, slack: U0PRI000 }
bridges:
  slack:
    from: ./.bulldog/bridges/slack
    places:
      "#help": { guests: true }        # anyone in #help may ask the office
      "#refunds": { team: payments }   # people in everyone ask payments
      direct: {}                       # people ask the office in a direct message
  gitlab:
    from: ./.bulldog/bridges/gitlab
    site: gitlab.com
    places:
      platform/api: { team: platform }
models:
  usual: { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }
people: [al]
lead: bot
heed: [./.bulldog/rules/for-us/for-us.yaml]
`,
		"bot.yaml": "agent: bot\nmodel: usual\nprompt: ${input.said}\ntools_file: [./.bulldog/tools/talk/talk.yaml]\n",
		"payments/team.yaml": `name: Payments
about: Charges, refunds and the ledger. Holds customer data.
people: [lee, priya]
lead: ledger
heed: [./.bulldog/rules/mentioned/mentioned.js]  # in #refunds, only what names the bot
check: [./no-account-numbers.js, lee, priya]     # may this leave payments?
decide: [lee, priya]                              # may this held job run?
`,
		"payments/ledger.yaml":           "agent: ledger\nmodel: usual\nprompt: ${input.said}\ntools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]\n",
		"payments/no-account-numbers.js": "return /\\d{8}/.test(input.answer ?? input.question.said) ? { says: 'no', note: 'An account number.' } : { says: 'yes', note: 'No account numbers.' }\n",
		"platform/team.yaml":             "name: Platform\nabout: The services and their deploys.\nlead: ops\n",
		"platform/ops.yaml":              "agent: ops\nmodel: usual\nprompt: ${input.said}\n",
	}
}

// with gives a copy of an office's files with some changed; an empty text
// removes the file.
func with(files map[string]string, changes map[string]string) map[string]string {
	out := maps.Clone(files)
	for name, text := range changes {
		if text == "" {
			delete(out, name)
		} else {
			out[name] = text
		}
	}
	return out
}

// write writes an office's files into a directory of the test's own.
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// read writes an office and reads it, and fails the test if the reading
// is refused.
func read(t *testing.T, files map[string]string) *Office {
	t.Helper()
	o, err := Read(write(t, files), shipped)
	if err != nil {
		t.Fatalf("the office is refused:\n%v", err)
	}
	return o
}

// refused writes an office and reads it, and gives the problems it is
// refused with.
func refused(t *testing.T, files map[string]string) Problems {
	t.Helper()
	o, err := Read(write(t, files), shipped)
	var problems Problems
	if !errors.As(err, &problems) || o != nil {
		t.Fatalf("the office is not refused: %v", err)
	}
	return problems
}

// has fails the test unless a problem is of the file, at the line when it
// is given, and holds every word.
func has(t *testing.T, problems []Problem, file string, line int, words ...string) {
	t.Helper()
	for _, p := range problems {
		if p.File == file && (line == 0 || p.Line == line) && !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(p.Text, w) }) {
			return
		}
	}
	t.Fatalf("no problem of %s at line %d says %q; the problems are:\n%v", file, line, words, Problems(problems))
}

func TestOfficeHouse(t *testing.T) {
	t.Parallel()
	o := read(t, house())
	if len(o.Problems) > 0 {
		t.Fatalf("the house has problems:\n%v", Problems(o.Problems))
	}
	if o.Name != "The house" || o.Zone.String() != "Europe/London" {
		t.Errorf("the office is %q in %v", o.Name, o.Zone)
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(o.ID) {
		t.Errorf("the id is %q", o.ID)
	}
	want := []Person{
		{ID: "sam", Name: "Sam", Identities: map[string]string{"telegram": "4471"}},
		{ID: "kit", Name: "Kit", Identities: map[string]string{"telegram": "4472"}},
	}
	if !reflect.DeepEqual(o.People, want) {
		t.Errorf("the people are %+v", o.People)
	}
	telegram := &Bridge{Name: "telegram", From: "./.bulldog/bridges/telegram", Dir: ".bulldog/bridges/telegram",
		Settings: map[string]any{"places": map[string]any{"direct": map[string]any{}}},
		Places:   []Place{{Name: "direct", Team: "office"}}}
	if len(o.Bridges) != 1 || !reflect.DeepEqual(o.Bridges[0], telegram) {
		t.Errorf("the bridges are %+v", o.Bridges)
	}

	office, garden := o.Team("office"), o.Team("garden")
	if len(o.Teams) != 2 || o.Teams[0] != office || o.Teams[1] != garden {
		t.Fatalf("the teams are %+v", o.Teams)
	}
	wantOffice := &Team{ID: "office", File: "office.yaml", Name: "The house", About: "The house and everything in it, but the garden.",
		People: []string{"sam", "kit"}, Lead: Lead{Worker: "steward"},
		Decide:   []Decider{{Rule: "./small-orders.js"}, {Worker: "clerk"}, {Person: "sam"}},
		Routines: []Routine{{At: "mon 09:00", Say: "What is still open?"}},
		Models:   map[string]map[string]any{"usual": {"name": "claude-sonnet-5-5", "secret": "ANTHROPIC_KEY"}},
		Workers: []Worker{
			{Name: "clerk", Type: "agent", File: "clerk.yaml", Team: "office"},
			{Name: "fixer", Type: "agent", File: "fixer.yaml", Team: "office"},
			{Name: "steward", Type: "agent", File: "steward.yaml", Team: "office"},
		}}
	if !reflect.DeepEqual(office, wantOffice) {
		t.Errorf("the office team is\n%+v, not\n%+v", office, wantOffice)
	}
	wantGarden := &Team{ID: "garden", Path: "garden", File: "garden/team.yaml", Within: "office", Name: "The garden",
		About: "The beds, the lawn and the shed.", People: []string{"sam"}, Lead: Lead{Worker: "gardener"},
		Routines: []Routine{{At: "fri 16:00", Say: "What needs doing before the weekend?"}},
		Models:   map[string]map[string]any{"usual": {"name": "claude-opus-5-5", "secret": "ANTHROPIC_KEY"}},
		Workers:  []Worker{{Name: "gardener", Type: "agent", File: "garden/gardener.yaml", Team: "garden"}}}
	if !reflect.DeepEqual(garden, wantGarden) {
		t.Errorf("the garden is\n%+v, not\n%+v", garden, wantGarden)
	}

	if w := o.Worker("gardener"); w == nil || w.Team != "garden" {
		t.Errorf("the gardener is %+v", w)
	}
	if p := o.PersonOn("telegram", "4472"); p == nil || p.ID != "kit" {
		t.Errorf("telegram 4472 names %+v", p)
	}
	if p := o.PersonOn("telegram", ""); p != nil {
		t.Errorf("no identity names %+v", p)
	}
	if m, _ := o.Model(garden, "usual"); m["name"] != "claude-opus-5-5" {
		t.Errorf("the garden's usual is %v", m)
	}
	if m, _ := o.Model(office, "usual"); m["name"] != "claude-sonnet-5-5" {
		t.Errorf("the office's usual is %v", m)
	}
	if got := office.MayAsk(); !reflect.DeepEqual(got, []string{"sam", "kit"}) {
		t.Errorf("the office may ask %v", got)
	}
}

func TestOfficeWork(t *testing.T) {
	t.Parallel()
	o := read(t, work())
	if len(o.Problems) > 0 {
		t.Fatalf("the work office has problems:\n%v", Problems(o.Problems))
	}
	if names := slices.Collect(func(yield func(string) bool) {
		for _, team := range o.Teams {
			yield(team.ID)
		}
	}); !reflect.DeepEqual(names, []string{"office", "payments", "platform"}) {
		t.Errorf("the teams are %v", names)
	}
	slack := o.Bridge("slack")
	places := []Place{{Name: "#help", Team: "office", Guests: true}, {Name: "#refunds", Team: "payments"}, {Name: "direct", Team: "office"}}
	if !reflect.DeepEqual(slack.Places, places) {
		t.Errorf("slack's places are %+v", slack.Places)
	}
	if gitlab := o.Bridge("gitlab"); gitlab.Settings["site"] != "gitlab.com" || gitlab.Places[0].Team != "platform" || o.Bridges[1] != gitlab {
		t.Errorf("gitlab is %+v", gitlab)
	}
	if got := o.Team("office").Heed; !reflect.DeepEqual(got, []Decider{{Rule: "./.bulldog/rules/for-us/for-us.yaml"}}) {
		t.Errorf("the office's heed is %+v", got)
	}
	payments := o.Team("payments")
	if !reflect.DeepEqual(payments.Heed, []Decider{{Rule: "./.bulldog/rules/mentioned/mentioned.js"}}) {
		t.Errorf("payments' heed is %+v", payments.Heed)
	}
	if !reflect.DeepEqual(payments.Check, []Decider{{Rule: "./payments/no-account-numbers.js"}, {Person: "lee"}, {Person: "priya"}}) {
		t.Errorf("payments' check is %+v", payments.Check)
	}
	if platform := o.Team("platform"); !reflect.DeepEqual(platform.People, []string{"al"}) || platform.Lead.Worker != "ops" {
		t.Errorf("platform is %+v", platform)
	}
	if p := o.PersonOn("slack", "U0DANA00"); p == nil || p.ID != "dana" {
		t.Errorf("slack U0DANA00 names %+v", p)
	}
	if p := o.PersonOn("gitlab", "U0DANA00"); p != nil {
		t.Errorf("gitlab U0DANA00 names %+v", p)
	}
}

func TestOfficeNotAnOffice(t *testing.T) {
	t.Parallel()
	problems := refused(t, with(house(), map[string]string{"office.yaml": ""}))
	has(t, problems, "office.yaml", 0, "holds no office.yaml", "not an office")
}

func TestOfficeID(t *testing.T) {
	t.Parallel()
	dir := write(t, house())
	files := func() []string {
		var names []string
		filepath.WalkDir(dir, func(file string, d os.DirEntry, err error) error {
			names = append(names, file)
			return err
		})
		return names
	}
	before := files()
	o, err := Read(dir, shipped)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(dir, ".bulldog", "id"))
	if err != nil || string(written) != o.ID+"\n" {
		t.Fatalf(".bulldog/id holds %q (%v), and the id is %q", written, err, o.ID)
	}
	if after := files(); !reflect.DeepEqual(after, slices.Concat(before[:1], []string{filepath.Join(dir, ".bulldog"), filepath.Join(dir, ".bulldog", "id")}, before[1:])) {
		t.Errorf("reading wrote more than .bulldog/id:\n%v\n%v", before, after)
	}
	again, err := Read(dir, shipped)
	if err != nil || again.ID != o.ID {
		t.Errorf("read again, the id is %q (%v), not %q", again.ID, err, o.ID)
	}

	problems := refused(t, with(house(), map[string]string{".bulldog/id": "Not/Kept\n"}))
	has(t, problems, ".bulldog/id", 0, "Not/Kept", "kept path")
}

func TestOfficeTeamsWithin(t *testing.T) {
	t.Parallel()
	o := read(t, with(house(), map[string]string{
		"garden/beds/team.yaml":   "name: The beds\nlead: sam\n",
		"tools/tidy.yaml":         "flow: tidy\nstages: {}\n",
		"tools/shed/team.yaml":    "lead: sam\n",
		"garden/notes/lawn.yaml":  "flow: lawn\n",
		".hidden/team.yaml":       "lead: sam\n",
		"garden/.bulldog/x.yaml":  "flow: x\n",
		"garden/beds/compost.txt": "not a worker\n",
	}))
	if len(o.Problems) > 0 {
		t.Fatalf("problems:\n%v", Problems(o.Problems))
	}
	var teams []string
	for _, team := range o.Teams {
		teams = append(teams, team.ID+" in "+team.Within+" at "+team.Path)
	}
	if want := []string{"office in  at ", "garden in office at garden", "beds in garden at garden/beds"}; !reflect.DeepEqual(teams, want) {
		t.Errorf("the teams are %q", teams)
	}
	if beds := o.Team("beds"); beds.Lead.Person != "sam" || !reflect.DeepEqual(beds.People, []string{"sam", "kit"}) || beds.Workers != nil {
		t.Errorf("the beds are %+v", beds)
	}
	for _, name := range []string{"tidy", "lawn", "x", "small-orders", "order"} {
		if w := o.Worker(name); w != nil {
			t.Errorf("%s is read as a worker: %+v", name, w)
		}
	}
}

func TestOfficeLoad(t *testing.T) {
	t.Parallel()
	files := house()
	o := read(t, files)
	for file, model := range map[string]string{
		"steward.yaml":         "{name: claude-sonnet-5-5, secret: ANTHROPIC_KEY}",
		"garden/gardener.yaml": "{name: claude-opus-5-5, secret: ANTHROPIC_KEY}",
	} {
		got, err := o.Load(file)
		if want := strings.Replace(files[file], "model: usual", "model: "+model, 1); err != nil || string(got) != want {
			t.Errorf("%s is loaded as\n%s(%v), not\n%s", file, got, err, want)
		}
	}
	if got, err := o.Load("small-orders.js"); err != nil || string(got) != files["small-orders.js"] {
		t.Errorf("small-orders.js is loaded as %q (%v)", got, err)
	}
	for name, want := range map[string]string{
		".bulldog/wait/wait.yaml":                            "wait/wait.yaml",
		"garden/.bulldog/tools/talk/talk.yaml":               "tools/talk/talk.yaml",
		"garden/.bulldog/tools/talk/.bulldog/wait/wait.yaml": "wait/wait.yaml",
		"garden/.bulldog/tools/talk/../../wait/wait.yaml":    "wait/wait.yaml",
	} {
		if got, err := o.Load(name); err != nil || string(got) != string(shipped[want].Data) {
			t.Errorf("%s is loaded as %q (%v), not as the shipped %s", name, got, err, want)
		}
	}
	if _, err := o.Load(".bulldog/rules/nothing.js"); err == nil || !strings.Contains(err.Error(), "Bulldog ships nothing at ./.bulldog/rules/nothing.js") {
		t.Errorf("loading what is not shipped gives %v", err)
	}
	if _, err := o.Load(".bulldog/id"); err == nil {
		t.Errorf("the office's id is served")
	}

	quoted := read(t, with(house(), map[string]string{"garden/gardener.yaml": "agent: gardener\nmodel: \"usual\"   # the garden's\nprompt: hi\n"}))
	if got, err := quoted.Load("garden/gardener.yaml"); err != nil || string(got) != "agent: gardener\nmodel: {name: claude-opus-5-5, secret: ANTHROPIC_KEY}   # the garden's\nprompt: hi\n" {
		t.Errorf("a quoted model is loaded as %q (%v)", got, err)
	}
}

// Corbie's parser takes a flow at the office's top whose work is the
// gardener: its tools file and the wait it names are what Bulldog ships,
// and its model is the garden's.
func TestOfficeLoadParses(t *testing.T) {
	t.Parallel()
	o := read(t, house())
	src := "flow: garden\ninput: { said: string }\nstages:\n  work:\n    job: { machine: ./garden/gardener.yaml, input: { said: \"${input.said}\" } }\n"
	draft, err := machine.Parse([]byte(src), o.Load)
	if err != nil {
		t.Fatal(err)
	}
	gardener := draft.Children["./garden/gardener.yaml"]
	if gardener == nil || !strings.Contains(string(gardener.Definition.Body), "claude-opus-5-5") {
		t.Fatalf("the gardener is %+v", gardener)
	}
	if len(gardener.Children) != 1 {
		t.Errorf("the gardener's tools name %d machines, not the shipped wait", len(gardener.Children))
	}
}

func TestOfficeNothingReadOutside(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "spy.js"), []byte("return { says: 'yes' }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "spy.yaml"), []byte("agent: spy\nmodel: usual\nprompt: hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	office := strings.Replace(house()["office.yaml"], "decide: [./small-orders.js, clerk, sam]", "decide: [./spy.js, clerk, sam]", 1)
	dir := write(t, with(house(), map[string]string{
		"office.yaml":      strings.Replace(office, "bridges:\n", "bridges:\n  away: { from: ../elsewhere/bridge }\n", 1),
		"garden/team.yaml": house()["garden/team.yaml"] + "check: [../../outside.js]\n",
	}))
	if err := os.Symlink(filepath.Join(outside, "spy.js"), filepath.Join(dir, "spy.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "spy.yaml"), filepath.Join(dir, "spy.yaml")); err != nil {
		t.Fatal(err)
	}

	o, err := Read(dir, shipped)
	if err != nil {
		t.Fatalf("the office is refused:\n%v", err)
	}
	has(t, o.Team("office").Problems, "office.yaml", 17, "./spy.js cannot be read")
	has(t, o.Team("office").Problems, "spy.yaml", 0, "escapes")
	has(t, o.Team("garden").Problems, "garden/team.yaml", 9, "../../outside.js", "leaves the office's directory")
	has(t, o.Bridge("away").Problems, "office.yaml", 8, "leaves the office's directory")
	if len(o.Bridge("telegram").Problems) > 0 {
		t.Errorf("telegram is stopped: %v", Problems(o.Bridge("telegram").Problems))
	}
	for _, name := range []string{"../outside.js", "garden/../../outside.js", "/etc/passwd", "spy.js", "spy.yaml"} {
		if got, err := o.Load(name); err == nil {
			t.Errorf("%s is loaded: %q", name, got)
		}
	}
}

func TestOfficeUnknownKeyRefused(t *testing.T) {
	t.Parallel()
	problems := refused(t, with(house(), map[string]string{
		"garden/team.yaml": house()["garden/team.yaml"] + "budget: 40\nzone: Europe/Paris\n",
	}))
	has(t, problems, "garden/team.yaml", 9, "budget is not a key of a team file")
	has(t, problems, "garden/team.yaml", 10, "zone is written in office.yaml only")

	problems = refused(t, with(house(), map[string]string{
		"office.yaml": strings.Replace(house()["office.yaml"], `{ name: Kit, telegram: "4472" }`, `{ name: Kit, telgram: "4472" }`, 1),
	}))
	has(t, problems, "office.yaml", 6, "telgram is not a bridge of the office")

	problems = refused(t, with(house(), map[string]string{
		"garden/team.yaml": house()["garden/team.yaml"] + "routines:\n  - { at: \"18:00\", say: Water, to: gardener }\n",
	}))
	has(t, problems, "garden/team.yaml", 9, "routines is written twice")
}

func TestOfficeIdentityNamesOnePerson(t *testing.T) {
	t.Parallel()
	problems := refused(t, with(house(), map[string]string{
		"office.yaml": strings.Replace(house()["office.yaml"], `telegram: "4472"`, `telegram: "4471"`, 1),
	}))
	has(t, problems, "office.yaml", 6, "telegram 4471 names both sam and kit")
}

func TestOfficeEveryProblemTogether(t *testing.T) {
	t.Parallel()
	problems := refused(t, with(house(), map[string]string{
		"office.yaml":      strings.Replace(house()["office.yaml"], "people: [sam, kit]", "people: [sam, jo]", 1),
		"garden/team.yaml": house()["garden/team.yaml"] + "budget: 40\n",
		"fixer.yaml":       "agent: fixer\n  model: [\n",
		"clerk.yaml":       strings.Replace(house()["clerk.yaml"], "model: usual", "model: fast", 1),
	}))
	has(t, problems, "office.yaml", 14, "people: jo is not a person")
	has(t, problems, "garden/team.yaml", 9, "budget")
	has(t, problems, "fixer.yaml", 0)
	has(t, problems, "clerk.yaml", 2, "fast")
	if len(problems) != 4 {
		t.Errorf("there are %d problems:\n%v", len(problems), problems)
	}
	if got := problems.Error(); !strings.Contains(got, "garden/team.yaml:9: budget is not a key") {
		t.Errorf("the problems are said as:\n%s", got)
	}
}

func TestOfficeBrokenWorkerStopsItsTeam(t *testing.T) {
	t.Parallel()
	o := read(t, with(house(), map[string]string{"fixer.yaml": "agent: fixer\n  model: [\n"}))
	has(t, o.Problems, "fixer.yaml", 0)
	has(t, o.Team("office").Problems, "fixer.yaml", 0)
	if len(o.Team("garden").Problems) > 0 {
		t.Errorf("the garden is stopped: %v", Problems(o.Team("garden").Problems))
	}

	o = read(t, with(house(), map[string]string{"fixer.yaml": strings.Replace(house()["fixer.yaml"], "agent: fixer", "agent: mender", 1)}))
	has(t, o.Team("office").Problems, "fixer.yaml", 1, "mender", "fixer")

	// A worker named as a decider stops the teams that name it.
	o = read(t, with(house(), map[string]string{
		"clerk.yaml":       "clerk: flow\n",
		"garden/team.yaml": house()["garden/team.yaml"] + "decide: [clerk]\n",
	}))
	has(t, o.Team("office").Problems, "clerk.yaml", 1, "agent, flow or statechart")
	has(t, o.Team("garden").Problems, "clerk.yaml", 1)
}

func TestOfficeLead(t *testing.T) {
	t.Parallel()
	o := read(t, with(house(), map[string]string{"garden/team.yaml": strings.Replace(house()["garden/team.yaml"], "lead: gardener", "lead: sam", 1)}))
	if lead := o.Team("garden").Lead; lead != (Lead{Person: "sam"}) {
		t.Errorf("the garden's lead is %+v", lead)
	}
	problems := refused(t, with(house(), map[string]string{
		"office.yaml":      strings.Replace(house()["office.yaml"], "lead: steward", "lead: gardener", 1),
		"garden/team.yaml": strings.Replace(house()["garden/team.yaml"], "lead: gardener\n", "", 1),
	}))
	has(t, problems, "office.yaml", 15, "gardener is a worker of garden")
	has(t, problems, "garden/team.yaml", 0, "garden has no lead")
}

func TestOfficeModels(t *testing.T) {
	t.Parallel()
	o := read(t, with(house(), map[string]string{"garden/gardener.yaml": strings.Replace(house()["garden/gardener.yaml"], "model: usual", "model: fast", 1)}))
	has(t, o.Team("garden").Problems, "garden/gardener.yaml", 2, "model: fast is not a model of garden")
	if len(o.Team("office").Problems) > 0 {
		t.Errorf("the office is stopped: %v", Problems(o.Team("office").Problems))
	}
	if _, err := o.Load("garden/gardener.yaml"); err == nil || !strings.Contains(err.Error(), "fast") {
		t.Errorf("loading the gardener gives %v", err)
	}
	// A definition that is not a worker takes the nearest team's model.
	o = read(t, with(house(), map[string]string{"garden/tools/weeder.yaml": "agent: weeder\nmodel: usual\nprompt: hi\n"}))
	if got, err := o.Load("garden/tools/weeder.yaml"); err != nil || !strings.Contains(string(got), "claude-opus-5-5") {
		t.Errorf("the weeder is loaded as %q (%v)", got, err)
	}

	problems := refused(t, with(house(), map[string]string{"garden/team.yaml": strings.Replace(house()["garden/team.yaml"], "routines:", "  fast: claude-haiku\nroutines:", 1)}))
	has(t, problems, "garden/team.yaml", 7, "models: fast is a model as Corbie writes one")
}

func TestOfficeRoutines(t *testing.T) {
	t.Parallel()
	o := read(t, with(house(), map[string]string{"garden/team.yaml": strings.Replace(house()["garden/team.yaml"], `"fri 16:00"`, `"18:00"`, 1)}))
	if got := o.Team("garden").Routines[0].At; got != "18:00" {
		t.Errorf("the routine is at %q", got)
	}
	problems := refused(t, with(house(), map[string]string{
		"office.yaml":      strings.Replace(house()["office.yaml"], "Europe/London", "Europe/Londn", 1),
		"garden/team.yaml": strings.Replace(house()["garden/team.yaml"], `"fri 16:00"`, `"25:00"`, 1),
	}))
	has(t, problems, "office.yaml", 3, "zone: Europe/Londn is not a time zone")
	has(t, problems, "garden/team.yaml", 8, "at: 25:00 is not a time")
}

func TestOfficeDeciders(t *testing.T) {
	t.Parallel()
	decide := func(list string) map[string]string {
		return with(house(), map[string]string{"office.yaml": strings.Replace(house()["office.yaml"], "decide: [./small-orders.js, clerk, sam]", "decide: "+list, 1)})
	}
	problems := refused(t, decide("[sam, ./small-orders.js, kit, clerk]"))
	has(t, problems, "office.yaml", 16, "./small-orders.js comes after sam", "people may only end a list")
	has(t, problems, "office.yaml", 16, "clerk comes after sam")
	problems = refused(t, decide("[nobody]"))
	has(t, problems, "office.yaml", 16, "nobody is neither a rule nor a worker nor a person")
	problems = refused(t, decide("./small-orders.js"))
	has(t, problems, "office.yaml", 16, "decide is written as a list")

	// A rule that cannot be read stops its team, and nothing else.
	o := read(t, decide("[./missing.js, ./.bulldog/rules/nothing.js, ./steward.yaml]"))
	has(t, o.Team("office").Problems, "office.yaml", 16, "./missing.js cannot be read")
	has(t, o.Team("office").Problems, "office.yaml", 16, "./.bulldog/rules/nothing.js cannot be read", "Bulldog ships nothing")
	has(t, o.Team("office").Problems, "office.yaml", 16, "./steward.yaml is a definition of the office's own")
	if len(o.Team("garden").Problems) > 0 {
		t.Errorf("the garden is stopped: %v", Problems(o.Team("garden").Problems))
	}

	// A rule's path is from its team file's directory.
	o = read(t, with(house(), map[string]string{"garden/team.yaml": house()["garden/team.yaml"] + "check: [../small-orders.js, ./.bulldog/rules/mentioned/mentioned.js, steward, kit]\n"}))
	want := []Decider{{Rule: "./small-orders.js"}, {Rule: "./.bulldog/rules/mentioned/mentioned.js"}, {Worker: "steward"}, {Person: "kit"}}
	if got := o.Team("garden").Check; !reflect.DeepEqual(got, want) {
		t.Errorf("the garden's check is %+v", got)
	}
	if got := o.Team("garden").MayAsk(); !reflect.DeepEqual(got, []string{"sam", "kit"}) {
		t.Errorf("the garden may ask %v", got)
	}
}

func TestOfficePeople(t *testing.T) {
	t.Parallel()
	problems := refused(t, with(house(), map[string]string{"office.yaml": strings.Replace(house()["office.yaml"], "people: [sam, kit]", "people: [sam, jo]", 1)}))
	has(t, problems, "office.yaml", 14, "people: jo is not a person of the office")
}

func TestOfficeNames(t *testing.T) {
	t.Parallel()
	problems := refused(t, with(house(), map[string]string{
		"2nd-floor/team.yaml": "lead: sam\n",
		"wait/team.yaml":      "lead: sam\n",
		"office.yaml":         strings.Replace(house()["office.yaml"], "  kit: {", "  chat: {", 1),
	}))
	has(t, problems, "2nd-floor/team.yaml", 0, "2nd-floor is not a name a team can have")
	has(t, problems, "wait/team.yaml", 0, "wait is reserved")
	has(t, problems, "office.yaml", 6, "chat is reserved")

	o := read(t, with(house(), map[string]string{"sam.yaml": "agent: sam\nmodel: usual\nprompt: hi\n"}))
	has(t, o.Team("office").Problems, "sam.yaml", 0, "sam is the name of a person already")
}

func TestOfficeBridges(t *testing.T) {
	t.Parallel()
	o := read(t, with(house(), map[string]string{
		"office.yaml": strings.Replace(house()["office.yaml"], "bridges:\n", `bridges:
  pager:
    places:
      "#on-call": { team: nowhere, guests: "yes" }
  slak: { from: ./.bulldog/bridges/slak }
`, 1),
		"bridges/pager/in.yaml": "statechart: pager-in\n",
	}))
	pager := o.Bridge("pager")
	if pager.Dir != "bridges/pager" || pager.From != "" {
		t.Errorf("pager is %+v", pager)
	}
	has(t, pager.Problems, "office.yaml", 10, "team nowhere is not a team")
	has(t, pager.Problems, "office.yaml", 10, "guests is true or false")
	has(t, pager.Problems, "office.yaml", 8, "./bridges/pager/out.yaml cannot be read")
	has(t, o.Bridge("slak").Problems, "office.yaml", 11, "./.bulldog/bridges/slak/in.yaml cannot be read", "Bulldog ships nothing")
	if len(o.Bridge("telegram").Problems) > 0 || len(o.Team("office").Problems) > 0 {
		t.Errorf("more is stopped than the bridges:\n%v", Problems(o.Problems))
	}
	if got := []string{o.Bridges[0].Name, o.Bridges[1].Name, o.Bridges[2].Name}; !reflect.DeepEqual(got, []string{"pager", "slak", "telegram"}) {
		t.Errorf("the bridges are in the order %v", got)
	}
}

func TestOfficeStop(t *testing.T) {
	t.Parallel()
	o := read(t, house())
	garden := o.Team("garden")
	p := Problem{File: "garden/gardener.yaml", Text: "tools.dig: no such tool"}
	o.Stop(garden, p)
	o.Stop(garden, p)
	if !reflect.DeepEqual(garden.Problems, []Problem{p}) || !reflect.DeepEqual(o.Problems, []Problem{p}) {
		t.Errorf("the garden's problems are %v, and the office's %v", garden.Problems, o.Problems)
	}
	if len(o.Team("office").Problems) > 0 {
		t.Errorf("the office is stopped")
	}
}

func TestOfficeAllow(t *testing.T) {
	t.Parallel()
	o := read(t, house())
	allow := o.Allow("garden")
	if err := permission.CheckList(allow); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(allow, "kept.read:"+o.ID+"/garden/**") || !slices.Contains(allow, "kept.write:"+o.ID+"/garden/**") || slices.Contains(allow, "all") {
		t.Errorf("the garden's runs are allowed %v", allow)
	}
	if got := o.Docs("garden"); got != o.ID+"/garden/docs" || permission.CheckKeptPath(got) != nil {
		t.Errorf("the garden's docs are at %q", got)
	}
}
