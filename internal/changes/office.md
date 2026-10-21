# How an office is written

An office is a folder of files, and the files are the office. It is the top team; every team inside it is a folder inside it. What runs is what is in the files, and nothing about the office is kept anywhere else.

There are two ways to write it. From outside, with nothing running, write the files in the folder: `corbie bulldog init <folder>` makes an office, and gives an empty folder the example house; `corbie bulldog check <folder>` reads it as the server would, and says every problem with its file; `corbie bulldog <folder>` serves it, and a served office reads a file again as soon as it changes. What is written this way is an edit by hand, the person's to commit. From inside, a builder in a served office drafts the whole new text of each file it touches, checks the draft and puts it, and it is written once its job runs: after the yes of the team whose folder holds it where the tools file marks `office_put` approve, as the build kit ships it, and at once where it does not.

```
house/
  office.yaml                  the top team's file
  steward.yaml  fixer.yaml     workers: machines directly in a team's folder
  crew.yaml                    a Corbie tools file the steward names
  small-orders.js              a rule
  tools/                       scripts the workers use
  notes/                       what the house knows, in Markdown
  garden/
    team.yaml  gardener.yaml   a team within the house
  .bulldog/                    Bulldog's own, which it writes: never edited
```

## A team

The top team's file is `office.yaml`; a team within is a folder directly inside a team's folder that holds a `team.yaml`. A team is known by its folder's name, and the top team as `office`.

```yaml
# office.yaml
name: The house
about: The house and everything in it, but the garden.
zone: Europe/London
everyone:
  sam: { name: Sam, title: Owner. Decides money and the building., telegram: "4471" }
  kit: { name: Kit, title: Lives here. }
models:
  usual: { name: sonnet, service: claude-code }
people: [sam, kit]
lead: steward
decide:
  everything: [sam]
  order_part: [./small-orders.js, clerk: advise, sam]   # the clerk looks at what the rule refers; Sam decides
routines:
  - { at: "mon 09:00", to: steward, say: "Walk round: what is still open?" }
```

| Key | What it is |
|---|---|
| `name` | what the team is called, in any words |
| `about` | one line saying what the team looks after; workers choosing whom to ask are given it |
| `people` | the team's people, by the names `everyone` writes them under |
| `lead` | the one worker or person of the team that answers for it |
| `heed` | one rule that weighs what is said at a channel place before the lead starts on it, as the section on a heed says |
| `decide` | ordered lists of deciders: `everything`; the name of a tool a worker of the team holds, a stage of a flow or a job of a statechart; and the kits' writes, `office_put`, `write_doc` and `write_notes`, for what is written into the team's folder |
| `disclosure` | a list of deciders for everything the team says outside itself |
| `routines` | what is said to one of its workers at a time of day, `"18:00"`, or with a weekday, `"mon 09:00"`, in the office's zone |
| `models` | models by a name of the team's, each written as Corbie writes one, naming the secret that holds its key and never a key |
| `zone`, `everyone`, `bridges`, `signin`, `runs` | the top team's only |

A decider is a person, a worker of any team by its name, or a rule by its path, in the team's own folder, as `./small-orders.js`, or among those Bulldog ships, in `.bulldog/rules/` at the top of the office. In a list every rule and worker comes before every person. A rule or a worker may be marked to advise, as `clerk: advise`: it may refuse, which ends the decision, or refer, but its yes never approves; it goes on to the next decider as a recommendation, as a referral would, and is shown as "clerk recommends yes" with its note. A person is never marked to advise. The same worker may advise on one list and decide plainly on another.

The people of the office, how each is reached and signs in, the sign-in methods, the top team's `office_put` list and its `everything`, a bridge's in machine, and a bridge's entry with its `from` and every setting but its places are a person's to write: a builder's draft that changes any of them is refused, and from outside you say what you would write there and let the person write it.

## A worker

A worker is a Corbie machine, an agent, a flow or a statechart, directly in a team's folder. Its name is its file's name, and its definition carries that name. A machine in any other folder, such as `tools/`, is something workers use. A model named by one word is the nearest team's of that name, its own team's first and the office's last.

```yaml
# weather.yaml
agent: weather
input: { said: string, context: string }
model: usual
system: You say what the weather will do here, and whether the washing can go out.
prompt: ${input.context}
tools_file: [./.bulldog/tools/talk/talk.yaml]
tools:
  forecast: { does: The forecast for the next few days, args: { days: "number?" },
              job: { script: ./tools/forecast.js, input: { days: "${args.days}" },
                     allow: [net:api.weather.example, "secret:WEATHER_KEY@net:api.weather.example"] } }
```

```js
// tools/forecast.js
const key = secrets.get("WEATHER_KEY")
const reply = http.get("https://api.weather.example/v1/forecast?days=" + (input.days || 3),
  { headers: { Authorization: secrets.text`Bearer ${key}` }, as: "text" })
if (reply.status < 200 || reply.status > 299) {
  throw new Error("The weather service answered " + reply.status + ": " + reply.body.slice(0, 200))
}
return JSON.parse(reply.body).days.map(d => d.date + ": " + d.summary)
```

A reply that is not a 2xx is no error to a script: ask for it as text, check its status, and parse it only after a 2xx. A tool that throws with the status and the start of the body keeps the service's reason, as "answered 401: Bad HTTP authentication header format". It goes back to the worker as the tool's error, and the team's people read it in the run's steps, and in Bulldog's entry to them when it fails the run, as a script a flow runs does; whoever asked is told only that the team could not answer.

Hold as a secret only what the service never sends back. An identifier it does send back, such as a client id, is written plainly, or masking spoils whatever carries it.

A worker has nothing of Bulldog's that its definition does not name, each by its path in `.bulldog/`: the talk tools, `tools/talk/talk.yaml`, for asking another team or a person, telling someone, and reading the thread; and three kits, each reaching its own team's files and those of the teams within it: the notes kit, `tools/notes/notes.yaml`, which gives each run its team's notes and writes them; the docs kit, `tools/docs/docs.yaml`, for reading the docs of its team and the teams around it, and mending its own team's with `write_doc`; the build kit, `tools/build/build.yaml`, which makes it a builder. `write_doc` and `office_put` are marked `approve: true` as Bulldog ships them, so they wait for a yes; `write_notes` is not. Each thing Bulldog ships is a folder there with its page of the manual, named by its path, as `tools/talk` or `bridges/slack`. A worker Bulldog starts at the top, a team's lead or a routine's worker, names the ask Bulldog ships when anything it runs can wait for a yes, a builder it calls among them: `ask: { machine: ./.bulldog/ask.yaml }`. From a team within, the paths begin `../.bulldog/`.

A tool whose job says `approve: true` waits for a yes, decided by the team's list named for the tool, or else its `everything`.

## A rule

A rule decides from what it is given, and gives back `{ says: yes | refuse | refer, note }`. A refer sends the decision on to the next decider with its note; the referrals before it are its input's `referrals`, each with its `says`, `refer`, or `yes` for the recommendation of one that advises. Its input's `advises` is true when its list marks it to advise: its yes then goes on as a recommendation to whoever its `referral_goes_to` names. A rule that needs nothing more is a bare script:

```js
// small-orders.js
const pounds = Number(input.action && input.action.args ? input.action.args.pounds : NaN)
if (pounds < 20) return { says: "yes", note: "Under £20." }
return { says: "refer", note: "£" + pounds + " is more than I order alone." }
```

A rule that needs anything, a host or a secret, is a Corbie definition, which holds its own permissions:

```yaml
# payments/no-account-numbers.yaml
flow: no-account-numbers
stages:
  check:
    job:
      script: ./no-account-numbers.js
      input: ${input}
      allow: [net:screen.example.com, "secret:SCREEN_KEY@net:screen.example.com"]
output: ${stages.check.output}
```

A held `office_put` or `write_doc` is decided by the innermost team whose folder holds every file it writes, by its list of the tool's name or else its `everything`; one that touches a team's `team.yaml` by the team around it, and `office.yaml` by the top team. A rule's yes approves it as any action; to have a person decide, put one on the list, and mark a check to advise: `office_put: [reviewer: advise, sam]`.

Rules build on each other in a list. A yes or a refusal ends the decision, but for the yes of one that advises, so a rule that finds something and leaves the judging to another refers, with what it found as its note, and every decider after it, rule or worker, is given that referral. A note may be JSON: it reaches the next rule as JSON text, and is shown to people as it was written. No decider changes what it decides: to have something taken out, a disclosure's rule refuses and says what, and the lead says it again.

```yaml
# payments/team.yaml
disclosure: [./finds-numbers.js, ./card-numbers.js]     # what both refer goes to Lee or Priya
```

```js
// payments/finds-numbers.js
const found = String(input.text).match(/\d{8,}/g) || []
if (found.length === 0) return { says: "yes", note: "No long numbers." }
return { says: "refer", note: { numbers: found } }
```

```js
// payments/card-numbers.js
const found = input.referrals.find((r) => r.by === "finds-numbers.js")
const numbers = found ? JSON.parse(found.note).numbers : []
const cards = numbers.filter((n) => n.length === 16)
if (cards.length > 0) return { says: "refuse", note: "Take out the card ending " + cards.map((n) => n.slice(-4)).join(", ") + "." }
return { says: "refer", note: "Long numbers, but no card." }
```

The second rule picks the first one's referral by its file's name, never by its place in the list, so another rule put before it changes nothing. This is the whole of its input, as payments' ledger answers Dana in the chat:

```json
{
  "kind": "disclosure",
  "decision": 1,
  "team": "payments",
  "team_called": "Payments",
  "does": "ledger's answer to Dana, in the chat on the screens",
  "text": "Yes: card 4111111111111111 was charged twice on 3 Oct. I refunded one.",
  "disclosure": {
    "what": "answer",
    "text": "Yes: card 4111111111111111 was charged twice on 3 Oct. I refunded one.",
    "to": "Dana",
    "read_at": "the chat on the screens",
    "said_in": 1,
    "notes": [{ "name": "refunds", "text": "# Refunds\n\nA charge taken twice within a minute is a duplicate. Refund the later one.\n" }]
  },
  "notes": [{ "name": "refunds", "text": "# Refunds\n\nA charge taken twice within a minute is a duplicate. Refund the later one.\n" }],
  "request": { "number": 1, "asked": "was customer 4411 charged twice?", "asker": "Dana" },
  "referrals": [{ "by": "finds-numbers.js", "kind": "rule", "says": "refer", "note": "{\"numbers\":[\"4111111111111111\"]}" }],
  "referral_goes_to": [{ "kind": "people", "list": "everything", "team": "payments", "to": "Lee or Priya" }],
  "advises": false,
  "ends": "Your yes or your refusal ends it. A referral goes on, with your note, to Lee or Priya, on Payments' everything list."
}
```

`what` is `answer`, `question`, `tell` or `sub`, a question to another team, which names it in `to_team` too. A decision on an action holds, in place of `text`, `disclosure` and `notes`, the `action`, with its `name`, `job`, `does` and `args`, the `worker` whose it is, and `said`, what the worker said with it; a held `office_put` or `write_doc` holds `change` beside, with who drafted it, why, every file, what it does and what it lets more by.

## A heed

A team may name one rule as its heed, which weighs what is said at a channel place before its lead starts on it. It answers `{ says: act | note, note }`: act, and the lead starts; note, and the message is kept in the request, marked noted, and nothing runs or is said. The lead is given noted messages in the thread when it next starts, and never the heed's note. A heed is no decider: nothing waits on it, nobody is asked, and it is asked only before a lead that is a worker.

```yaml
# in a team file
heed: { rule: ./.bulldog/rules/for-us/for-us.yaml, threshold: 0.6 }
# or, with no settings
heed: ./.bulldog/rules/mentioned/mentioned.js
```

It is written as any rule is, a bare script or a definition, in the team's folder or among those Bulldog ships: `mentioned`, which acts on what names the bot or follows straight on its answer, and `for-us`, which asks a fast model of the rest; each has its page. Every key beside `rule` is a setting, handed to the rule whole; Bulldog reads none of them, and keeps no time limit of its own: for-us's is its `timeout`. A heed that is a definition gives its settings in its `input:` under `settings`, each with its `default` and `description`, as for-us does: Corbie fills in each default before it starts, and the screens show each setting, as written or as its default. It weighs only messages said at the request's own place by a person or a guest: a direct message, the chat, a routine's request, a sub-request, a reply to what the lead put to someone, and the opening of an issue start the lead with no heed. A heed that fails, runs out of time or answers anything else counts as act, and the team's people are told.

It is given `said`, each message it weighs on its line; `request`, the request as its lead would be given it, with those messages as `new`, each marked `mentioned` when its bridge says it names the bot; and `settings`, `{}` for a heed named by its path alone. This is the whole of it, as Dana asks in #help:

```json
{
  "said": "[Mon 5 Oct 2026 10:00 BST] Dana to Platform support: Raj, did the deploy go out?",
  "settings": { "threshold": 0.6 },
  "request": {
    "number": 1,
    "now": "2026-10-05T10:00:00+01:00",
    "zone": "Europe/London",
    "team": {
      "name": "office", "called": "Platform support",
      "about": "Help with the platform and with payments, asked in #help.",
      "people": [{ "name": "al", "called": "Al", "title": "Runs the office." }]
    },
    "worker": "bot",
    "workers": [],
    "teams": [
      { "name": "payments", "called": "Payments", "about": "Charges, refunds and the ledger. Holds customer data." },
      { "name": "platform", "called": "Platform", "about": "GitLab, the pipelines and deploys." },
      { "name": "support", "called": "Support", "about": "Customers' questions, and what they are told." }
    ],
    "asker": { "kind": "person", "name": "dana", "called": "Dana", "teams": ["support"] },
    "origin": { "kind": "place", "bridge": "slack", "place": "#help/1700.1" },
    "readers": [{ "kind": "person", "name": "dana", "called": "Dana", "teams": ["support"] }],
    "guests_reading": 0,
    "any_guest_reading": true,
    "answer_goes_to": "Your last reply is your answer, said to Dana on slack, in #help/1700.1. An empty reply says nothing.",
    "entries": [],
    "entries_not_shown": 0,
    "new": [{
      "id": 1, "kind": "asked", "time": "2026-10-05T10:00:00+01:00",
      "by": { "kind": "person", "name": "dana", "called": "Dana", "bridge": "slack", "id": "U0DANA00" },
      "to": [{ "kind": "team", "name": "office", "called": "Platform support" }],
      "text": "Raj, did the deploy go out?", "noted": false, "mentioned": false
    }],
    "subs": [],
    "notes": [],
    "notes_not_shown": "",
    "said": "Raj, did the deploy go out?"
  }
}
```

## A bridge

A bridge is where things come in and go out. Each is named, with its settings, under `bridges` in `office.yaml`, and runs a folder with an in machine and an out machine: one Bulldog ships, which its `from` names, as `./.bulldog/bridges/slack`, used in place and kept current with Bulldog; or, with no `from`, the office's own folder of its name in `bridges/`. The settings list the places it listens in, each with the team asked there, the office when none is named, and whether guests may ask there. Each shipped bridge's page, as `bridges/slack`, gives its other settings.

```yaml
# in office.yaml
bridges:
  slack:
    from: ./.bulldog/bridges/slack
    places:
      "#help": { guests: true }
      "#deploys": { team: platform }
```

A bridge of the office's own names no `from`, and its machines are in `bridges/<name>/`, such as:

```yaml
# bridges/slack/out.yaml
flow: slack-out
input: { settings: any, to: string, text: blob }
stages:
  say:
    job:
      script: ./slack.js
      input: { do: say, carry: "${input}" }
      allow: [net:slack.com, "secret:SLACK_TOKEN@net:slack.com"]
output: ${stages.say.output}
```

An in machine listens or hears, and the input it takes says which. One that takes `cursor` listens: Bulldog keeps one run of it going, which asks the service what is new since the cursor and ends with what it heard and the cursor to go on from, so nothing said between its runs is missed. One that takes `delivery` hears: the service calls the office at `<office url>/bridges/<name>` with each thing said or pressed, and the in machine checks with the secret it holds that it is the service's. A service that only pushes, keeping nothing to read back from, must be heard, and must be able to reach that address. A Slack app that is heard names the address in both of its settings that call it, Event Subscriptions and Interactivity, or a choice pressed on a decision never arrives. The bridges Bulldog ships all listen.

The in machine says who a person is, and so is a person's to write. So is a bridge's entry, its `from` and every setting but its places, since they say what it runs, where it listens and whom it signs in as; a builder may draft a change to its places.

## Signing in

A person signs in to the screens by any way their entry in `everyone` writes. Everyone may sign in with a one-time link sent over their bridges. An entry that writes their account by a sign-in method of the office, under the method's name, lets them sign in by it; `password: true` lets them set a password of their own on their Me page, which is never in the files. Each sign-in method is named, with its settings, under `signin` in `office.yaml`, and runs one machine: one Bulldog ships, which its `from` names, as `./.bulldog/signin/google` or `./.bulldog/signin/entra`, or, with no `from`, the office's own `signin/<name>/signin.yaml`. Its machine is given the code the provider sent the browser back with, takes it to the provider with the client's secret, which only it holds, and answers the account; Bulldog signs in the one person whose entry writes that account by the method, and nobody else. A method's name is never a bridge's, nor `name`, `title` or `password`. Each shipped method's page, as `signin/google`, gives its settings and what to set up at the provider: the address it sends the browser back to, `<office url>/signin/<name>/back`, and the secret Corbie keeps.

```yaml
# office.yaml
name: Platform support
everyone:
  al:  { name: Al, slack: U0AL0000, entra: al@platform.example, password: true }
  raj: { name: Raj, slack: U0RAJ000, entra: raj@platform.example }
  sam: { name: Sam, slack: U0SAM000, google: sam@example.com }
signin:
  entra:
    from: ./.bulldog/signin/entra
    tenant: 11111111-2222-3333-4444-555555555555   # the Directory (tenant) ID
    client_id: 5f3c1a2e-0b7d-4e8a-9c61-2a4b8d0e7f19
  google:
    from: ./.bulldog/signin/google
    client_id: 1234567890-abc.apps.googleusercontent.com
bridges:
  slack: { from: ./.bulldog/bridges/slack, places: { "#help": {} } }
# Corbie keeps ENTRA_CLIENT_SECRET and GOOGLE_CLIENT_SECRET; no secret is written here.
```

How a person signs in is a person's to write, as how they are reached is: their accounts, `password`, and each sign-in method, its entry and its folder. A builder's draft that changes any of them is refused; from outside, say what you would write and let the person write it.

## Notes

A team's notes are the files in its `notes/` folder, prose in Markdown. Each run of a team's worker that names the notes kit is given them, and one that names it not is given none. They are not private: a worker that names the build kit, of the team or one around it, reads them, and so does whoever decides a change. A secret belongs in Corbie's secrets, never in a note.

## Docs

A team's docs are the Markdown files under its `docs/` folder, subfolders included, each named by its path there without `.md`, as `refunds/policy`: reference people write, through a change, or a worker mends with `write_doc`. No run is given them: a worker whose definition names the docs kit reads them when it asks, its own team's first, then those of each team around it, out to the office, the nearer team's doc of a name standing before one further out.

## Drafting a change, as a builder

A builder changes a served office with the builder's tools, never in the folder.

1. `office_files` lists every file you reach, your own team's and those of the teams within it, and what your draft does to each.
2. `office_read` a file before you write it; `office_write` puts its whole new text in your draft, and `office_remove` has your draft remove it.
3. `office_check` reads the office as your draft would make it: every problem, Corbie's among them, and what the change does as whoever decides will be told, with what it lets more by apart. Mend every problem it says.
4. `office_put` with why, its first line saying what the change is. It waits for the yes of the team whose folder holds what it writes, the team around it for a `team.yaml`; whoever decides is shown every file whole, and what the change does read from the files alone. You are told when it is written, or not written, with why, and a refusal is said as the call denied, with the note.

Your draft is kept with the request you work on, from one run to the next, until it is written.
