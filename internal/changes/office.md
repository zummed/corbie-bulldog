# How an office is written

An office is a folder of files, and the files are the office. It is the top team; every team inside it is a folder inside it. What runs is what is in the files, and nothing about the office is kept anywhere else.

There are two ways to write it. From outside, with nothing running, write the files in the folder: `corbie bulldog init <folder>` makes an office, and gives an empty folder the example house; `corbie bulldog check <folder>` reads it as the server would, and says every problem with its file; `corbie bulldog <folder>` serves it, and a served office reads a file again as soon as it changes. What is written this way is an edit by hand, the person's to commit. From inside, a builder in a served office drafts the whole new text of each file it touches, checks the draft and puts it to be decided, as the last section says; only a person's yes writes it.

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
  order_part: [./small-orders.js, sam]
routines:
  - { at: "mon 09:00", to: steward, say: "Walk round: what is still open?" }
```

| Key | What it is |
|---|---|
| `name` | what the team is called, in any words |
| `about` | one line saying what the team looks after; workers choosing whom to ask are given it |
| `people` | the team's people, by the names `everyone` writes them under |
| `lead` | the one worker or person of the team that answers for it |
| `decide` | ordered lists of deciders: `everything`; the name of a tool a worker of the team holds; and at the top, `change`, for a change to the office files |
| `disclosure` | a list of deciders for everything the team says outside itself |
| `routines` | what is said to one of its workers at a time of day, `"18:00"`, or with a weekday, `"mon 09:00"`, in the office's zone |
| `models` | models by a name of the team's, each written as Corbie writes one, naming the secret that holds its key and never a key |
| `zone`, `everyone`, `bridges`, `runs` | the top team's only |

A decider is a person, a worker of any team by its name, or a rule by its path, in the team's own folder, as `./small-orders.js`, or among those Bulldog ships, in `.bulldog/rules/` at the top of the office. In a list every rule and worker comes before every person.

The people of the office, how each is reached, who decides a change to the office, a bridge's in machine, and a bridge's entry with its `from` and every setting but its places are a person's to write: a builder's draft that changes any of them is refused, and from outside you say what you would write there and let the person write it.

## A worker

A worker is a Corbie machine, an agent, a flow or a statechart, directly in a team's folder. Its name is its file's name, and its definition carries that name. A machine in any other folder, such as `tools/`, is something workers use. A model named by one word is the nearest team's of that name, its own team's first and the office's last.

```yaml
# weather.yaml
agent: weather
input: [said, context]
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
  { headers: { Authorization: secrets.text`Bearer ${key}` }, as: "json" })
return reply.body.days.map(d => d.date + ": " + d.summary)
```

A worker has nothing of Bulldog's that its definition does not name, each by its path in `.bulldog/`: the talk tools, `tools/talk/talk.yaml`, for asking another team or a person, telling someone, and reading the thread; the notes tool, `tools/notes/notes.yaml`, for writing its team's notes; the docs tools, `tools/docs/docs.yaml`, for reading the docs of its team and the teams around it; the builder's tools, `tools/build/build.yaml`. Each thing Bulldog ships is a folder there with its page of the manual, named by its path, as `tools/talk` or `bridges/slack`. A worker Bulldog starts at the top, a team's lead or a routine's worker, names the ask Bulldog ships when anything it runs can wait for a yes: `ask: { machine: ./.bulldog/ask.yaml }`. From a team within, the paths begin `../.bulldog/`.

A tool whose job says `approve: true` waits for a yes, decided by the team's list named for the tool, or else its `everything`.

## A rule

A rule decides from what it is given, and gives back `{ says: yes | refuse | refer, note }`. A refer sends the decision on to the next decider with its note; the referrals before it are its input's `referrals`. A rule that needs nothing more is a bare script:

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

On a change to the office, a rule's yes is taken as a referral: only a person approves a change.

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
input: [settings, to, text]
stages:
  say:
    job:
      script: ./slack.js
      input: { do: say, carry: "${input}" }
      allow: [net:slack.com, "secret:SLACK_TOKEN@net:slack.com"]
output: ${stages.say.output}
```

The in machine says who a person is, and so is a person's to write. So is a bridge's entry, its `from` and every setting but its places, since they say what it runs, where it listens and whom it signs in as; a builder may draft a change to its places.

## Notes

A team's notes are the files in its `notes/` folder, prose in Markdown. Every run of the team's workers is given them. They are not private: the builder reads them, and so does whoever decides a change. A secret belongs in Corbie's secrets, never in a note.

## Docs

A team's docs are the Markdown files under its `docs/` folder, subfolders included, each named by its path there without `.md`, as `refunds/policy`: reference people write, through a change. No run is given them: a worker whose definition names the docs tools reads them when it asks, its own team's first, then those of each team around it, out to the office, the nearer team's doc of a name standing before one further out.

## Drafting a change, as a builder

A builder changes a served office with the builder's tools, never in the folder.

1. `office_files` lists every file, and what your draft does to each.
2. `office_read` a file before you write it; `office_write` puts its whole new text in your draft, and `office_remove` has your draft remove it.
3. `office_check` reads the office as your draft would make it: every problem, Corbie's among them, and what the change does as whoever decides will be told, with what it lets more by apart. Mend every problem it says.
4. `office_put` with why, its first line saying what the change is. Whoever decides is shown every file whole, and what the change does read from the files alone. You are told when it is written, refused, or not written, with why.

Your draft is kept with the request you work on, from one run to the next, until it is written.
