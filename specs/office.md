# Office

The office is a folder. It is the top team, and every team inside it is a folder inside it. Bulldog reads the folder, has Corbie check every definition in it, with any model named by one word filled in from the nearest team that names it, and goes by what it read until the files change. Nothing about the office is kept anywhere but its files.

## The folder

The house, as a person writes it, with the Telegram bridge Bulldog ships set by Sam as its first change, with the settings Bulldog ships it with:

```
house/
  office.yaml                  the top team's file
  steward.yaml  fixer.yaml  sparks.yaml  clerk.yaml  bins.yaml  builder.yaml
  crew.yaml                    a Corbie tools file: the steward's colleagues
  small-orders.js              a rule
  tools/                       scripts the workers use
  notes/                       what the house knows
  garden/
    team.yaml  gardener.yaml  groundsman.yaml
    notes/
  .bulldog/                    Bulldog's own
```

- **The top team's file is `office.yaml`.** A team within is a folder directly inside a team's folder that holds a `team.yaml`, and it may hold teams of its own the same way.
- **A worker is a machine directly in a team's folder**: a file whose definition is an agent, a flow or a statechart. Its name is its file's name, and its definition carries that name. A machine in any other folder, such as `tools/`, is something workers use, and not a worker.
- **`notes/`** in a team's folder holds its notes, and **`docs/`** its docs. A bridge runs the folder Bulldog ships that its entry's `from` names, as `./.bulldog/bridges/telegram`, or else the office's own: **`bridges/`**, at the top only, holds a folder for each bridge of the office's own, `bridges/<name>`, found by the bridge's name alone. The settings `bridges` gives a bridge are handed whole to its machines, and Bulldog reads of them only the folder `from` names and the places the bridge listens in, whose shape is the bridges page's; the office checks that each team a place names is a team of the office, and that the settings fit the shape its machines' input gives them. A sign-in method runs a folder the same way: the one Bulldog ships that its entry's `from` names, as `./.bulldog/signin/google`, or else the office's own, **`signin/<name>`**, at the top only, holding its one machine, `signin.yaml`.
- **`.bulldog/` is Bulldog's, not the office's.** It holds Bulldog's store, the office's identity, and what Bulldog ships, which a definition names by its path there, and Corbie reads it from there as it reads the office's files. What Bulldog ships is its own machines, flat at the top, the ask among them; and one folder for each thing an office uses, a tools file in `tools/<name>/`, a rule in `rules/<name>/`, a bridge in `bridges/<name>/` or a sign-in method in `signin/<name>/`, with the thing's page, `README.md`, which is also its page of the manual, named by its path, as `tools/talk`. No change touches it and it is not in the history. Nothing else whose name starts with a dot is the office either.

The office the server writes into an empty folder is this house without the bridge and its sign-in by Google: no `bridges` or `signin` key and no Telegram identities or Google accounts under its people, spoken to in the built-in chat alone. It reads with no problem at once; the server page says when it is written.

Everything a definition or a team file names is inside the folder. What runs is what is in the files. A folder that is not there, or holds no `office.yaml`, is not an office, and Bulldog writes nothing into it.

## A team

```yaml
# office.yaml
name: The house
about: The house and everything in it, but the garden.
zone: Europe/London
everyone:
  sam: { name: Sam, title: Owner. Decides money and the building., telegram: "4471", google: sam@example.com, password: true }
  kit: { name: Kit, title: Lives here., telegram: "4472" }
bridges:
  telegram:
    from: ./.bulldog/bridges/telegram
    places:
      direct: {}                   # as Bulldog ships it: Sam and Kit may ask the house in a direct message
signin:
  google:
    from: ./.bulldog/signin/google
    client_id: 1234567890-house.apps.googleusercontent.com   # from the Google Cloud console; Corbie keeps GOOGLE_CLIENT_SECRET
models:
  usual: { name: sonnet, service: claude-code }
people: [sam, kit]
lead: steward
decide:
  everything: [sam]
  order_part: [./small-orders.js, clerk: advise, sam]   # the clerk looks at what the rule refers; Sam decides
routines:
  - { at: "mon 09:00", to: steward, say: "Walk round: what is still open?" }
  - { at: "18:00", to: bins, say: "Which bin goes out tonight?" }
```

```yaml
# garden/team.yaml
name: The garden
about: The beds, the lawn and the shed.
people: [sam]
lead: gardener
decide: { everything: [sam] }
models:
  usual: { name: opus, service: claude-code }   # the garden's own
routines:
  - { at: "fri 16:00", to: gardener, say: "What needs doing before the weekend?" }
```

| Key | What it is |
|---|---|
| `name` | what the team is called, in any words. The top team's is the office's name |
| `about` | one line saying what the team looks after. Workers choosing whom to ask are given it, and the screens show it |
| `people` | the people of the team, by name |
| `lead` | the one worker or person of this team that answers for it |
| `heed` | one rule that weighs what is said at a channel place before the lead starts on it: its path, as `./for-us.js`, or `{ rule: ./for-us.yaml, timeout: 5s }`, every key beside `rule` a setting handed to the rule whole |
| `decide` | who decides what, as ordered lists of deciders: `everything`; the name of a tool one of its workers holds, a `list:` entry such as `github` among them, or of one tool of an entry, such as `github_create_issue`; and the kits' writes, `office_put`, `write_doc` and `write_notes`, in any team, since a team decides the writes into its folder whoever makes them |
| `disclosure` | an ordered list of deciders for everything the team says to anyone outside it: its answers, questions and tells that anyone outside it reads, someone the office cannot name counting as outside, and the requests it makes of other teams. Without one, all of it leaves unchecked |
| `routines` | what is said to its workers at set times |
| `models` | models by name, each a model written as Corbie writes one, which names the secret that holds its key, where it needs one, and never holds a key. A worker that names its model by one word is given the nearest team's model of that name |
| `zone`, `everyone`, `bridges`, `signin`, `runs` | the top team's only: the office's time zone, in which times are shown and routines' `at` read, UTC if not given; every person of the office, each written once, whether or not any team names them; its bridges with their settings, among them the folder Bulldog ships that each runs and the places each listens in; its sign-in methods with their settings, among them the folder Bulldog ships that each runs; and how many runs may be at work at once in the whole office, 3 if not given; a run waiting on a person, another team or a decision is not at work |

**A team is known by its folder's name, and the top team as `office`.** Its `name` is only what it is called: nothing is found by it, so it may be in any words and change freely. A folder renamed is one team gone and another new.

**A team's people are exactly those its `people` names.** Being in a team around it puts nobody in it. A team may be all people, all workers or both. A team that writes no `everything` has its people as that list, and one with no people has an empty one; how its decisions then go out to the team around it is the decisions page's, and that the people they reach are its people for every purpose of sight and control is the requests page's.

**A decider is a person, a worker of any team, or a rule.** A rule is named by its path from the team's file, and is in the team's own folder, or is one Bulldog ships, which any team may name by its path in `.bulldog/rules/`, as `./.bulldog/rules/<name>/<file>` from `office.yaml` and `../.bulldog/rules/<name>/<file>` from a team directly within. A rule that needs nothing but what it is given is a bare script, such as `small-orders.js`. A rule that needs anything more, a network, a host or a secret, is a Corbie definition, which says in its own allow lines what it may reach, as any definition does; a definition a list names by its path is a rule wherever it sits, and never a worker. Bulldog grants a rule nothing. In a list, every rule and worker comes before every person, and a list may name no person at all. A rule or a worker may be marked to advise on a list, written as a mapping of its name or path to the one word `advise`, as `[reviewer: advise, paul]`: it may refuse, or refer, but its yes is a recommendation and goes on, as the decisions page says. Any other marking, and a person marked to advise, since a person never refers, is a problem of the team file at its line. The same worker may advise on one list and decide plainly on another. A rule a list names that still answers `pass` or reads `passes`, as rules did before `refer`, has each such line of its files reported, with its file and line, on each team whose lists name it, and still runs. How the lists are used is the decisions page's.

**A team's `heed` is one rule**, named by its path as a list names one, in the team's own folder or among those Bulldog ships, and read and checked as any rule is. Written as a mapping, every key but `rule` is a setting, handed to the rule whole and as written; Bulldog reads none of them, and a heed written as a path alone has none, but the office checks them against the shape its rule's input gives `settings`, and settings that do not fit stop the heed. A heed is no decider: nothing waits on it, and no list names it. It is asked only before a lead that is a worker, since everything a person lead says is already its answer: a heed in a team whose lead is a person is a problem of the heed alone, which is not asked, and the team reads. Bulldog ships two heeds, `mentioned` and `for-us`, as the work page says.

**The top team's `everything` names a person**, as written or as taken from its people. It is where every decision that nobody else takes ends, and it is the one check of the reading that a person is reached.

## People

Every person is written once, under `everyone` in `office.yaml`, by a name of their own. Beside `name` and `title`, which are what they are called and what they are to the office, each key is a bridge's name or a sign-in method's. A bridge's gives who they are on that bridge, by the lasting id the service gives them and never a handle they can change: `telegram: "4471"`, `slack: U0DANA00`. Their bridges are read in the order written, which is the order a one-time sign-in link is tried in, as the server page says. A person with no `name` is called by the name they are written under. Every person is also reached on the screens. Most people never sign in: they live in their bridges, where they ask, are answered and decide.

**A person signs in to the screens by any way their entry writes.** Every person may sign in with a one-time link sent over their bridges, as the server page says. A sign-in method's name gives who they are by that method, as its machine answers it, such as their address at Google: `google: sam@example.com`, `entra: al@corp.example`, compared whatever its case. `password: true` lets them set a password of their own once signed in; the password itself is never in the files.

```yaml
# office.yaml
signin:
  google:
    from: ./.bulldog/signin/google
    client_id: 1234567890-abc.apps.googleusercontent.com
everyone:
  sam: { name: Sam, telegram: "4471", google: sam@example.com, password: true }
```

`signin` names each sign-in method of the office, with its settings, as `bridges` names each bridge. A method runs one machine, which is given the code a provider gave a person's browser and answers who they are there, holding the provider's secret itself; the server page says how. An account the office does not write signs nobody in, whatever it is. Bulldog ships Google's and Microsoft Entra ID's, each with its page; Bulldog reads of a method's settings only the folder `from` names and those the server page says it builds the provider's address from, and the office checks that the settings fit the shape its machine's input gives them.

**How a person is reached is their entry in the files**, so changing it is a change to the office, decided like any other, their own entry as much as anyone's. Changing one's own identity on a bridge is what someone who had taken over an account would do first.

Two people never share who they are on one bridge, since a yes is taken from the person that identity names, and never share an account by one sign-in method, whatever its case, since whoever holds it signs in as the person it names. Someone a bridge names who is written nowhere is a guest, and is not written in the office.

## Workers

A worker is sent to Corbie as its file is written. What it may touch, its secrets, which of its actions need a yes and its model are all in its definition, as Corbie reads them, and Bulldog rewrites nothing in it but a model named by one word, as below.

A worker writes its model out in full, as Corbie writes one, or names it by one word, `model: usual`. A model written as one word is always a name, never a model of Corbie's: it is looked up in the `models` of the worker's own team, then of each team around it, out to the office, and the nearest team that gives the name gives the model, so a team may give a name a model of its own. Putting that model in place of the word is the one thing Bulldog fills into a definition; it is what Corbie checks and runs, and what the screens show. A name no team gives is a problem of that worker, which is given no run until it is mended, and the rest of the office reads. A rule's definition and a bridge's machines name a model the same way, a bridge's looked up from the office. A rule Bulldog ships names no model by one word and holds its own allow lines: it is sent as it ships, the same rule whichever team names it, and is of no team. Like any rule, it decides as the team whose decision it is: its run is labelled with that team, and its word in the thread is that team's reading.

```yaml
# fixer.yaml
agent: fixer
model: usual
input: { said: string }
prompt: ${input.said}
system: You do the repairs. Before anything electrical, ask sparks which circuit, and cut it.
tools_file: [./.bulldog/tools/talk/talk.yaml]
tools:
  sparks:    { does: Which circuit is what, args: { question: string },
               job: { machine: ./sparks.yaml, input: { said: "${args.question}" } } }
  cut_power: { does: Turn one circuit off, args: { circuit: string },
               job: { script: ./tools/cut-power.js, input: { circuit: "${args.circuit}" }, approve: true } }
  order_part: { does: Order a part, args: { part: string, pounds: number },
               job: { script: ./tools/order.js, input: { part: "${args.part}", pounds: "${args.pounds}" }, approve: true } }
```

Which tools a worker holds, and so what a `decide` key may name, is read from its definition as Corbie's client fills it out; for a flow, its stages, and for a statechart, its jobs, each by the name Corbie gives the job it runs: a state's one job by the state's name, and each of its named jobs by its own. So is what each one's job is, told from where it resolves from the worker's folder: a worker of the same team, another machine or a script, each a file of the office, a machine written in place, or one of the tools Bulldog ships, whose job is the call it ships, `.bulldog/call.yaml`. A path in a tools file is read from the tools file's own folder, as Corbie reads every path. A worker a flow's stage or a statechart's job runs is called by that flow or statechart, as a colleague is called as a tool.

The fixer's `usual` is the house's; the gardener's `usual` would be the garden's.

Inside a team, workers use each other as ordinary Corbie tools, as the fixer uses sparks. A worker has nothing of Bulldog's that its own definition does not name, each by its path in `.bulldog/`, and Bulldog writes none of it into a definition. A worker that asks or tells another team or a person, or reads its thread, names the talk tools Bulldog ships, `tools/talk/talk.yaml`, in its `tools_file:`. Beside them Bulldog ships three kits, each a tools file, and a worker's Bulldog tools reach its own team's files and those of the teams within it: one that knows and writes its team's notes names the notes kit, `tools/notes/notes.yaml`; one that reads and mends docs names the docs kit, `tools/docs/docs.yaml`; and one that drafts the office's files names the build kit, `tools/build/build.yaml`, which makes it a builder, whatever it is called. A team's notes are the team's, and each run Bulldog starts of a worker that names the notes kit is given them; a worker that names it not is given none, and a colleague called as a tool is given what its caller gives it.

Corbie gives a held action to the nearest ask at or above the machine that holds it. So a worker Bulldog starts at the top, a team's lead or a routine's worker, names the ask Bulldog ships, `ask.yaml`, as its `ask:` when anything it runs can be held: an action of its own, or of a colleague or any other machine its tools run, as far down as they go. A colleague called as a tool uses the nearest ask above it unless it names the ask Bulldog ships itself, which it may, and must when it can also be started at the top. In the house the steward names the ask, since the fixer it calls holds `cut_power` and the builder it calls holds `office_put`, which the build kit marks for a yes; the fixer and sparks, only ever called as tools, name none, and the fixer's held `cut_power` goes to the ask the steward names. A worker that must name the ask and does not, or that names another, is reported as its own problem and given no run.

## Routines

A routine is something said to one of a team's workers at a set time. `at` is a time of day as Corbie reads one, `"18:00"`, or with a weekday before it, `"mon 09:00"`. Like every time the files write, it is written without a zone and read in the office's `zone`, or in UTC when the office names none. `to` is a worker of the same team, and `say` is what it is told. A routine that should come on several days is written once for each. Where its answer goes is the work page's.

## Notes

A team's notes are the files in its `notes/` folder: prose the team knows, in Markdown. Notes are the team's, never one worker's. They are read from the folder each time they are needed, and each run Bulldog starts of one of the team's workers that names the notes kit is given them in its context, as the work page says. A team that sets a disclosure check may say there, in plain words, what it never lets out.

A worker writes its own team's notes with `write_notes` of the notes kit, and has it only when its own definition names that file; a team that does not want its workers writing its notes names it in none of their definitions. What a run may write with it is the work page's. As the kit ships, a note is written at once, with no yes: a note is what a team knows, not how the office works; an office that wants its notes decided marks `write_notes` with `approve: true` in its own copy of the kit. Each write is a commit that says who wrote it and why, as the changes page says.

Notes are files of the office like any other. They are not sealed as what a team read is: they are in the history, and a worker that names the build kit, of the team or one around it, and whoever decides a change read them. On the screens a team's people see its notes and its problems, and the people who decide changes see every file, note and problem of the office, as the server page says. A secret belongs in Corbie's secrets, and what a team read belongs in its runs, never in its notes.

## Docs

A team's docs are the Markdown files under its `docs/` folder, subfolders included: reference its people keep, such as a policy or a runbook, each named by its path there without `.md`, as `refunds/policy`. A file or folder whose name starts with a dot, and anything that is not Markdown, is no doc. Docs are read from the folder each time they are needed, and are written through a change, like any file of the office, or by `write_doc` of the docs kit, which writes one doc of the worker's own team and, as the kit ships, waits for a yes from that team's deciders.

Where a run is given its team's notes, no run is given docs. A worker reads them when it asks, with the docs kit, `tools/docs/docs.yaml`, and has it only when its own definition names that file. It reads its own team's docs, then those of each team around it, out to the office, as a model named by one word is looked up: the nearer team's doc of a name stands before one further out, and each doc it is given names its team. A team's docs are not sealed as what it read is, being files of the office; which of them a run read is in the run's record, which is. On the screens a team's docs are opened by its people, the people of each team within it, whose workers read them, and the people who decide changes, as the server page says.

## At work

```
work/
  office.yaml  bot.yaml  docs/help/refunds.md
  platform/  team.yaml  ops.yaml  tools/
  payments/  team.yaml  ledger.yaml  tools/  no-account-numbers.yaml  no-account-numbers.js  notes/
  support/   team.yaml
  .bulldog/
```

```yaml
# office.yaml
name: Platform support
about: "Help with the platform and with payments, asked in #help."
zone: Europe/London
everyone:
  al:    { name: Al, title: Runs the office., slack: U0AL0000, gitlab: "1041", entra: al@platform.example }
  dana:  { name: Dana, title: Support., slack: U0DANA00, entra: dana@platform.example }
  lee:   { name: Lee, title: Payments., slack: U0LEE000, entra: lee@platform.example }
  priya: { name: Priya, title: Payments., slack: U0PRI000, entra: priya@platform.example }
  raj:   { name: Raj, title: Backend., slack: U0RAJ000, gitlab: "1187", entra: raj@platform.example }
bridges:
  slack:
    from: ./.bulldog/bridges/slack
    places:
      "#help": { guests: true }
  gitlab:
    from: ./.bulldog/bridges/gitlab
    site: gitlab.com
    places:
      platform/api: { team: platform }
signin:
  entra:
    from: ./.bulldog/signin/entra
    tenant: 72f988bf-86f1-41af-91ab-2d7cd011db47       # the directory's ID; Corbie keeps ENTRA_CLIENT_SECRET
    client_id: 5f3c1a2e-0b7d-4e8a-9c61-2a4b8d0e7f19
models:
  usual: { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }
people: [al]
lead: bot
heed: ./.bulldog/rules/for-us/for-us.yaml
decide: { everything: [al] }

# platform/team.yaml: one worker, on its own, with no people
name: Platform
about: GitLab, the pipelines and deploys.
lead: ops

# payments/team.yaml
name: Payments
about: Charges, refunds and the ledger. Holds customer data.
people: [lee, priya]
lead: ledger
decide: { everything: [lee, priya] }
disclosure: [./no-account-numbers.yaml]    # what it refers goes to Lee or Priya

# support/team.yaml: people only
name: Support
about: Customers' questions, and what they are told.
people: [dana]
lead: dana
```

Al is in the office but in no team within it, so he sees inside none of them. Raj is in no team at all: he is a person, who may ask and be named to decide. Platform has no people and writes no `decide`, so what it holds for a yes goes out to the office's `everything`, Al, and Al is its people for sight and control: he is told when it fails, sees what it read, and watches and stops its runs.

The office heeds with `for-us`, as Bulldog ships it and with its settings' defaults, so what is said in #help that names neither the bot nor follows straight on its answer is weighed by a fast model before the bot starts on it. The bot names the talk tools and the docs kit, and writes its own `ask_team` in full beside them, taking only platform and payments, so it never asks support, whose lead is Dana. The docs kit's `write_doc` waits for a yes, so the bot names the ask:

```yaml
# bot.yaml
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]
ask: { machine: ./.bulldog/ask.yaml }
tools:
  ask_team:
    does: Ask platform or payments, and wait for its answer
    args:
      type: object
      properties:
        team: { type: string, enum: [platform, payments] }
        question: { type: string }
      required: [team, question]
    job:
      machine: ./.bulldog/call.yaml
      input: { tool: ask_team, args: "${args}", unanswered: … }
```

The office's docs hold `help/refunds`, what #help is told of refunds, which the bot reads when it asks, and mends with `write_doc` once Al says yes; payments' notes say how its own workers find a duplicate, and the ledger names the notes kit to be given them.

Payments' rule must reach the screening service with a key, so it is a definition: `no-account-numbers.yaml` is a flow whose one job runs `no-account-numbers.js`, and its own allow lines let that job reach the service's host with the key bound to it, and nothing else. Being named by its path in `disclosure`, it is a rule and not a worker of payments.

## Reading the office

Bulldog reads the whole folder at once. Every team file is read, every definition in the office, workers' and bridges' machines and rules, is checked by Corbie as it would be checked before a start, with a model named by one word filled in, and every problem found is reported together, each with its file and what is wrong. Checking needs no running Corbie, no network and no model.

There are two kinds of problem.

- **A problem in a team file refuses the reading.** A file that is not well formed, a key a team file does not take or a top-only key in a team within, an `about` of more than one line, a name that is not allowed, used twice or reserved, a name in a list that is nobody, a list's entry marked with anything but `advise` or a person marked to advise, a `lead` that is not of its team, a decider that is none of a person, a worker of any team, a rule in the team's own folder or a rule Bulldog ships, a `heed` that names no such rule by its path, a rule or worker after a person in a list, an office whose `everything` names no person, a `runs` that is not a whole number above nought, a routine that is not right, a `models` entry that is not a mapping or whose name is not allowed, a path that leaves the folder or names nothing, or a team file where no team can be: any of these, and the office goes on as it was last read well.
- **A broken worker, rule or bridge stops only itself, and the workers that run it.** Corbie checks a definition with every colleague it runs as a tool, so a worker whose colleague is refused is refused too, and is reported once as using it. A worker whose definition is refused, whose one-word model no team gives, or that does not name the ask it must, is given no run until it is mended, and when it is a lead its team is reported too; a refused rule is reported on each team that names it, and a heed whose rule is refused, whose settings do not fit its rule's input, or whose team's lead is a person, is reported on its team and is not asked. A broken bridge, an entry in `bridges` with no folder at `bridges/<name>` and no `from`, a `from` that names no folder Bulldog ships under `.bulldog/bridges/`, an entry with `from` and a folder at `bridges/<name>` too, a folder there that `bridges` does not name, a folder missing either machine, an in machine that takes both `cursor` and `delivery`, the inputs the bridges page names, or neither, a machine Corbie refuses, settings that are not a mapping or do not fit its machines' input, or places that are not right, a team that is not there among them, is a problem of that bridge shown on the office, and the bridge carries nothing until it is mended. A sign-in method is the same: an entry in `signin` with no folder at `signin/<name>` and no `from`, a `from` that names no folder Bulldog ships under `.bulldog/signin/`, an entry with `from` and a folder at `signin/<name>` too, a folder there that `signin` does not name, a folder with no `signin.yaml`, a machine that does not take `code`, `verifier`, `redirect`, `nonce` and `settings`, a machine Corbie refuses, and settings that are not a mapping or do not fit its machine's input, are a problem of that method shown on the office, and the method signs nobody in until it is mended. A `decide` key that names no tool, stage or job of the team's workers is reported on its team and used for nothing. The rest of the office reads and runs.

The problems shown are always those of the latest reading. Until the office has once been read well, it starts nothing and only reports. No reading is kept from one start to the next: a file broken by hand while the server is stopped leaves the office unread at the next start, with every problem said, until it is mended.

**The office is read again whenever any of its files changes**: written, added, removed or renamed. A good reading replaces the last one whole, and a change reaches a worker at its next run.

**Names** of people, workers and teams are small letters, figures and dashes, beginning with a letter, and each is used once in the whole office. Bridges and models are named the same way; a model's name is a team's, and a team within may give it again. These names are reserved, because they are files and folders of the office or who Bulldog itself speaks as: `office`, `team`, `notes`, `docs`, `bridges`, `signin`, `chat`, `routine`, `bulldog`, `guest`. Sign-in methods are named as bridges are, and no method has a bridge's name, so a key of a person says which of the two it is. No bridge or method may be called `name`, `title` or `password` either, and `chat` is the built-in chat, which is not written.

## Interfaces

- **Requests** read who is in each team, who each person is on each bridge, each team's lead and the team each bridge's place names, to say who sees a request and where it goes, and say who stands as the people of a team with none: its own, or those its decisions reach.
- **Work** starts each team's lead and its workers from their definitions as read, with a one-word model filled in, keeps to the office's `runs`, starts each routine's clock with the office's zone, gives workers each team's `about`, gives each run it starts of a worker that names the notes kit the team's notes, ships the kits, each of which a worker has only when its definition names its file, the docs kit reading a team's docs and those of each team around it, and is told when a reading replaces the last, to start, stop and cancel what changed.
- **Decisions** reads the `decide` and `disclosure` lists and the notes a disclosure check reads, and runs the rules they name, each as it is written. The office gives the people a team's decisions reach: those its `everything` names, or its people where it writes none, or, where that names nobody, those of the first `everything` outward that does, with anyone no longer in the office left out. Decisions and Requests both take them from here.
- **Bridges** reads `bridges`, the folders under `bridges/` and the folders Bulldog ships that a `from` names, says the shape of the places a bridge listens in, whose teams the office checks, and matches the identity a bridge gives to the person whose entry names it.
- **Changes** writes the office's files, a person's entry among them, keeps each note a worker writes as a commit, has every change checked by a reading of the folder as it would be after the change, and asks which files are the office's and which files only one worker's definition names.
- **Server** reads the office from the folder it is given at each start, writes the house without its bridge or Google into an empty folder, holds the office's identity, which is not its name and not in its files, signs people in, sending a one-time link over a person's bridges in the order written, by the sign-in methods `signin` names, each to the person whose entry writes the account its machine answers, and by a password of their own where their entry says `password: true`, and shows each team's `about`, notes, docs and problems of the latest reading, and each worker's model as filled in, with what each of its tools may reach, each model's secret, and when each routine comes round next, to those who may see them.
- **Corbie's client** checks every definition, as it does before a start.
