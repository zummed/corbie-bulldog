# Server

Bulldog is one program, `corbie-bulldog`, which `corbie bulldog` runs. It serves one office: it readies the folder, opens the office's store, reads the office, takes the address it listens on, and only then starts the office's work. It signs people in, and gives the screens what each person may see and do. How the screens look is not said here.

```sh
$ corbie bulldog
bulldog: there was no office in /home/sam/bulldog, so the example was written there. It is yours to change: start with office.yaml
bulldog: /home/sam/bulldog is a git repository now, and what is there is its first commit
bulldog: on the Corbie of the controller laptop-controller, whose store is /home/sam/.config/corbie/store
bulldog: The house: 2 teams, 8 workers, 2 people
bulldog: listening on 127.0.0.1:7410
bulldog: sam, open http://127.0.0.1:7410/?key=4f1c…
bulldog: kit, open http://127.0.0.1:7410/?key=9a2e…
```

## The program

| Command | What it does |
|---|---|
| `corbie bulldog [folder]` | serves the office in the folder |
| `corbie bulldog check [folder]` | reads the folder as the server would, starts nothing and writes nothing, and says what it found and everything wrong |
| `corbie bulldog init [folder]` | readies the folder as the server would, without serving it, and says what it did; in an office that was moved, takes the move |
| `corbie bulldog version` | prints which Bulldog this is, the Corbie it is built on, and the machine types and plugins that Corbie holds, as `corbie version` does |
| `corbie bulldog manual [page]` | lists the manual's pages, or prints one: Bulldog's, `office`, one for each thing Bulldog ships and `bulldog`, then Corbie's and a page for each command |
| `corbie bulldog mcp [--allow <command>]…` | serves the manual and the commands allowed to a model, as an MCP server |

With no folder named, it is the folder the program is run in when that holds an `office.yaml`, and otherwise `bulldog` in the person's home. A folder is readied the same way by every command that readies one:

| What it finds | What it does |
|---|---|
| a folder with nothing in it, or `~/bulldog` not there | writes the example office there, which is in the program, and makes it a git repository of its own, even inside another |
| an office in no repository | makes it one, leaving `.bulldog/` out, and commits what is there |
| an office with no `.bulldog/` | makes it, with the office's identity |
| an office whose `.bulldog/` was made in another folder | refuses it, naming both folders, unless the command is `init`, which takes it as the office moved |

The example is the house the office page shows, without its bridge: no `bridges` key and no Telegram identities under its people, spoken to in the built-in chat alone. It reads with no problem at once, and a person can ask it something as soon as it is served. Setting a bridge is a person's first change: setting the shipped Telegram bridge writes `telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }` under `bridges`, the settings Bulldog ships it with, which hear people's direct messages to the bot.

- A folder named that is not there is refused, and nothing is made: `corbie bulldog init <folder>` makes it.
- A folder that has things in it and no `office.yaml` is no office. It is refused, and left as it was.
- When there is nothing to make, it says only what it serves.

**`mise run demo` shows the house with nothing set up.** It serves the house the office page shows, with its Telegram bridge, on a Corbie in the same process, with a stand-in model and a stand-in for the Telegram bridge, the tests' own, so it needs no network, no key and no Corbie set up on the computer. It is the quickest way to see the screens at work. The stand-in plays the house's workers through the house's day, each knowing its part by what it is and by the words it was asked. A socket that sparked goes to the fixer, who asks sparks which circuit, cuts it once Sam says yes, orders a £12 socket, which the small-orders rule says yes to, and asks whoever asked to say when it comes; once they have, the fixer says it is fitted, and the steward tells them so. If the cut is refused, the fixer says the circuit is still on and touches nothing; if the order is, that the socket stays off until one is ordered. A fan that stopped goes to the fixer too, whose £48 order the rule refers to Sam with its note; on his yes the fan is ordered, and on his no the fixer says it is not. The lawn, the garden, the shed and the hedge are asked of the garden, whose gardener hands the heavy work to the groundsman, and the steward passes on what the garden says, or what came back in its place; money goes to the clerk; and the walk round says what is open. Anything else is answered with a line saying what the demo plays. The demo opens with the first part of that day played, as Sam and Kit would on the screens, each step once the one before it has settled. Kit asks about the kitchen socket, with a photo of it, and Sam says yes to cutting its circuit, which leaves it waiting on Kit. Sam asks about the bathroom fan, which waits on his yes to the order. Kit asks whether the lawn is all right after the rain, which the garden answers. Sam runs the bins routine, whose answer is the bin that goes out tonight, and is empty on a night with none. What stops the opening is said in the log, and the house is served all the same.

`corbie bulldog` reads its command line with Corbie's own program for one, so its help, its mistakes and its exit statuses are Corbie's. `check` and `init` are words before they are folders: `./check` names a folder.

`manual` and `mcp` are Corbie's, given to the program, so a model learns to write an office from the one binary. The manual is the one a builder reads with its `manual` tool: Bulldog's page `office`, on how an office is written, the page of each thing Bulldog ships, named by its path, as `tools/talk` or `bridges/slack`, and its page `bulldog`, what Bulldog is and how a model writes an office for a person; then Corbie's pages. A model connected to `mcp` is told the page `bulldog`, reads any page with the tool `manual`, and runs the commands it is allowed with the tool `bulldog`:

```
corbie bulldog mcp --allow check --allow init     an assistant may check and ready an office
```

What a model writes in the folder is an edit by hand, the person's to commit, as the changes page says. Bulldog's own pages of the manual, and none of Corbie's, are on the screens too, for everyone signed in.

## Which Corbie

**Bulldog is a client of the Corbie set up on this computer**, and runs no role. `corbie start controller runner bulldog` runs the controller and the runner with Bulldog beside them, and when one of the three stops, all stop. Another Corbie is named as `corbie` itself is told: `--config` or `CORBIE_CONFIG` for the folder a client was set up in, `--store` or `CORBIE_STORE` for its controller's store. Bulldog needs nothing else of Corbie: no gateway, no token, and no address of its own for jobs to call.

The server says which Corbie it is on before anything else, with the store's address shown without any name, password or setting it carries. A config whose client has no identity of its own cannot start runs, and is refused before anything is served, saying what to run.

## Where it is served

`--listen`, or `BULLDOG_LISTEN`, is the address the server listens on: `127.0.0.1:7410` when neither is given. `--url`, or `BULLDOG_URL`, is how a person reaches it, and every link the office gives is made with it: `http://` and the listen address when neither is given. Both say where an office is served, not what it is, so neither is in the office's files, and a change to either is taken at the next start. The server speaks plain HTTP; a proxy in front of it gives it TLS.

## Starting and stopping

**The server takes the address it listens on before it starts any work.** It readies the folder, writes the files Bulldog ships, opens Corbie's client and the store, makes sure no other server is serving the office, asking Corbie alone and before anything of the office is opened or written, and reads the office; then it listens; and only then does it start its own run, and watching Corbie, the bridges, the clocks and the reading of the office again. An address that cannot be taken starts nothing.

**One server serves an office at a time.** While it serves, the server keeps one run of its own on the Corbie, labelled with the office's identity, which it renews every twenty seconds and which ends itself when a minute passes without one; a server that stops ends it. A server that finds such a run of its office at its start waits for it, up to a minute: if it is renewed, another server is serving the office, and the start is refused, naming the controller and saying what to do: stop the server already serving it, or, for a copy meant as an office of its own, remove its `.bulldog/`. If it ends, the server that left it is gone, and the start goes on. If it is neither renewed nor ended once its minute has passed, and a little more, no controller is running it, and the start is refused, saying so.

An office that does not read well is served all the same. A problem of one worker or one bridge does not keep it from reading well: that one is stopped and its problem shown, and the rest of the office reads and runs; a bridge's problem is the office's, shown to the top team's people. Its problems are in the log, each with its file and what is wrong, and nobody signs in until it reads well, since who the people are is in its files. No reading is kept from one start to the next, so a file broken by hand while the server was stopped leaves the office unread at the next start until it is mended. Once it has read well, a later reading that fails leaves it served as it was last read well, with the problems shown.

The log says which Corbie, the office's name with how many teams, workers and people it has, every problem of the reading, and a link for each person. At the first start of a new version of Bulldog, it also says, of each thing Bulldog ships that the office uses, each line of its page's Changed newer than the version that ran before. Each later reading adds its new problems, and a link for each person new in it. A link is printed only once every area has taken the reading it comes from, so that whoever opens it at once finds the office served as it read.

A signal stops the server: it stops listening, stops what it started, waits for all of it, closes the store, and ends as having done what it was asked. Nothing it needs is kept only in memory, so it can be stopped at any moment and started again.

## Bulldog's own folder

```
~/house/.bulldog/
    identity        the office's identity, and the folder it was made in
    bulldog.db      the store
    ask.yaml        the ask
    call.yaml       the call every tool Bulldog ships runs, which only waits
    live.yaml       the server's own run, which says it is serving
    clock.yaml      a routine's clock, given its time
    tools/
      talk/         talk.yaml  README.md      the talk tools
      notes/        notes.yaml  README.md     the notes tool
      docs/         docs.yaml  README.md      the docs tools
      build/        build.yaml  README.md     the builder's tools
    bridges/
      telegram/  slack/  gitlab/  jira/       in.yaml  out.yaml  <name>.js  settings.yaml  README.md
    rules/
      mentioned/    mentioned.js  README.md                 a heed that asks no model
      for-us/       for-us.yaml  for-us.js  README.md       a heed that asks a fast model
```

Bulldog's own machines sit flat at the top: those it starts or answers itself, with nothing an office would change. Each other thing it ships is one folder, with its page, `README.md`: its first line says what it is, then what it needs, its settings, what it leaves out, an example of using it and what changed in it with each version of Bulldog. Each page is also a page of the manual, named by the thing's path, as `tools/talk`. A bridge's `settings.yaml` is its entry under `bridges` in `office.yaml`, as it is shipped.

A bridge runs the shipped folder its entry's `from` names, as `./.bulldog/bridges/telegram`, and a team names a rule shipped here as it names one of its own, as its heed `./.bulldog/rules/for-us/for-us.yaml`. A worker names what it uses of these by its path here: the talk tools, the notes tool, the docs tools or the builder's tools in its `tools_file:`, as `./.bulldog/tools/talk/talk.yaml`, and the ask as its `ask:`. A worker Bulldog starts at the top names the ask when anything it runs can be held; a colleague called as a tool uses the nearest ask above it unless it names the ask Bulldog ships itself, and must name it when it can also be started at the top. Every ask a worker names is this one, as the work page says.

- **The office's identity** is made once, at random, when the folder is first readied. It is how every run of the office is labelled and found, so two offices of one name on one Corbie are two offices, and an office keeps its runs when it is renamed. It is kept beside the store and not in it, so a store made again finds the runs the office still has.
- **The identity knows the folder it was made in.** A copy of an office made with its `.bulldog/` would be the same office to Corbie, and the two would answer and cancel each other's runs; so a folder whose `.bulldog/` was made in another, by its real path with links followed, is refused before its store is opened, naming both, and saying what to do: remove `.bulldog/` for an office of its own, or run `corbie bulldog init` in it if the office was moved. `init` then takes the folder as the office's, keeping its identity, its store and its runs, and says so. `check` reads such a folder all the same, since it starts nothing.
- **The store** is one file, and each area keeps its own part of it. A store made by an older Bulldog is refused, saying so, and the server does not start; removing it starts the office again with nothing kept.
- **The version that ran before** is the one fact the store keeps of what Bulldog ships, beside the version serving the office now. At the first start of a new version, the log says what changed since it in each thing the office uses, and the people who decide a change are shown what changed in each thing while that version serves the office. Whether the office uses a thing, or holds a copy of it, is worked out from its files each time it is shown.
- **The files Bulldog ships** are written at every start, whole, as the program has them, so they always name what the program answers. The folders of the things it ships are written afresh, so no file an earlier version shipped stays in them, and the tools files an earlier version shipped flat at the top are taken away; the identity and the store are never touched.
- `.bulldog/` is not the office, is never in its history, and no change touches it.

## Signing in

**Every person has a key.** It is made when they are first in a reading of the office, and kept in the store. The link the server prints carries it: opening `/?key=…` gives the browser the key as a cookie and moves on to `/` without it. The cookie is the only way the key is shown after that. It is Secure exactly when the browser reached the server over TLS, directly or through a proxy that says so. A person who leaves the office loses their key, and someone written later under the same name is given a new one. A person may replace their own key, which signs out every browser that held it. The key is the store's, not the files', so replacing it is no change.

**Most people never sign in**: they live in their bridges, asking, answering and deciding there, and the screens are for those who want them. How a person is reached is their entry in the files, the identity each bridge gives for them, and it is where a one-time link goes; so changing it, their own entry included, is a change decided like any other, and nothing on the screens or said on a bridge sets it. Changing one's own identity on a bridge is what someone taking over an account would do.

**A person who has no link asks for one** on the sign-in page, by their name. A link that works once, for ten minutes, is sent to their direct place on one bridge: the first of their bridges, in the order their entry names them, that sends it. The out machine of the first is started; if it does not send, the next is tried, and so on until one sends or none is left. A bridge with no direct place sends none, so the next is tried. The page says the same thing whatever name it was given. A person is sent at most one in a minute. Opening the link shows a page, and pressing on it signs them in, since a service that carries a link opens it to show a preview.

The link is no entry of any request, and is said in no thread: whoever reads a request must never be handed a way to sign in as someone else. Signing in uses no service outside the office. A guest is not a person, has no key, and never signs in.

## What a person sees and may do

What each person may see is read from the files each time it is shown: being named in a team is what lets someone see into it. Who sees a request, which part of it, who may say more in it and what waits on whom are the requests area's to say, and the server asks it. Who may stop a request, and who may put a change from the screens, are the server's to say, and are said here.

**A team with no people** has the people its decisions reach as its people, for every purpose of sight and control, as the requests area says, and wherever this page says a team's people it means them too: in the work office, Al is told when platform fails, sees what it read, and watches and stops its runs.

**The people who decide a change** are those the changes area names: the people the top team's `change` list names; where it names no person, the people its `everything` names; and where neither names a person, the top team's people.

| Everyone signed in | |
|---|---|
| sees | the office's name; how many runs it lets work at once, how many are at work and how many requests wait their turn; who decides a change, in the order the walk asks them; each bridge with the folder it runs, whether it listens or hears, and where, whether it is running or stopped, and each place it listens in, with the team asked there and whether guests may ask there; each team with what it is called, its `about`, its people, or, for a team with none, the people its decisions reach, its lead and whether it can be started, and whether it names a heed; each person with their name, their title and the bridges that reach them; Bulldog's pages of the manual, `office`, the page of each thing Bulldog ships and `bulldog`; each thing Bulldog ships, with its page, the hosts and secrets it reaches and its settings, each with what it is when not written, as shipped, whether the office uses it, and the bridges that run it |
| sees, of themselves | who they are on each bridge, in the order a one-time link is tried, and whether each bridge has a direct place to send it to; each list of deciders that names them, with where on it they are; and each team whose decisions end at them |
| sees | the requests they may see, by team, by state and by the place they were asked at, those whose every message was noted only when they ask for them, with how many there are; a request that waits its turn shown queued, with its place in the queue; those they asked; what waits on them, and how much; of each, the teams and people it asked, where its answer stands, and each held action and change in it, what it is, how it stands and who ended it |
| asks | in the chat, of the office or of any team |
| says more | on the screens, in a request they asked, one whose team they are in, one whose origin they can read, or a sub-request asked by a team they are in |
| answers | a question put to them |
| decides | a decision the walk reaches them for, while it waits, shown the note of each decider that referred it and its walk, and, of a disclosure, what would leave, whom it is to, where it may in the end be read and the requests it goes through |
| stops | a request they asked, one whose team they are in, or one beneath a request they may stop |
| replaces | their own key |

| The people of a team | |
|---|---|
| see | its team file as read, among it its heed's rule, with its settings, each as the team file writes it or, where it leaves one out, the default the rule's input gives it, marked as the default, or none when it gives none, and its lists and its disclosure, each with who decides, in the order the walk asks them out to the people it ends at; each rule it names, with how it names it, what it reaches and whether it can be started; each model it gives, with the secret it names; each of its workers with its type, its model as it is sent, a model named by one word shown filled in from the nearest team that gives it, with the secret it names, what its file writes of how it runs, an agent's system and prompt, its inputs, its tools files and whether it names the ask, the files of the office it reads, and its tools, a flow's stages or a statechart's jobs, each with what it does and what its job is: a worker of the team, by name, another machine or a script, by its file, with what its job may reach, is refused and is asked about, or one of the tools Bulldog ships; a `list:` entry as the tools its server lists, named after it; which of them wait for a yes and who decides, in the order the walk asks them out to the people it ends at, a rule by its path in the office and a worker with the team it is of; the colleagues it calls, as far down as they go, whose jobs that wait for a yes come to its ask; the teams it may ask, as the work page says; its routines, each in the office's zone, with when it comes round next and whether it is due, let go by while the request it last opened is still open, or cannot run; its notes, each with its size and whether a run is given it whole, cut or not at all; its problems; which of its workers are at work, and on what; what was read in it, and its runs as they happen, each with its kind, its request and the worker or rule it is of, a heed's with what it said and its word on each message it noted; of a run of its lead or of a routine's worker, what it was given, worked out again from the request and the files when it is shown and kept nowhere: the team's notes, each with its size and whether it was given whole, cut or not at all, the docs it read, the teams it may ask, the thread it was shown, with how many entries were left out, and what was new for it; of each of its requests, its disclosure, whether its answer needs it and who decides it, and the notes, walk and arguments of its decisions; which of its workers hold the tools of a tools file Bulldog ships, and where it names a rule Bulldog ships |
| run | one of its routines now |

A run is the team's its labels name. A worker of one team asked to decide for another runs as the team it decides for, labelled with it, as the decisions area says, and is watched by that team's people, not its own team's. A run of a team that has gone, or has been renamed, is watched by nobody, and so is a bridge's run, whose record holds what was said in every place it listens in.

| The people who decide a change | |
|---|---|
| see | every file of the office, notes and docs included, every team as it is written, each bridge's settings and the hosts and secrets its in and out machines reach, every problem, the history, and which files are edited by hand; of each thing Bulldog ships, what changed in it since the version that ran before, the office's copies of a bridge with the files that differ from what ships now, and what each bridge that runs it reaches as its settings fill it; and nothing of what a team they are not in is doing or has read |
| draft, check and put | a change of their own, which may set a bridge Bulldog ships by its entry, or start from Use or Copy of a thing Bulldog ships, and which still goes first to every rule and worker the walk asks before it reaches a person |

Anyone else who wants the office changed asks for it in a request, and a builder drafts it.

**Each thing Bulldog ships** is used or unused, and, to the people who decide a change, copied. It is used when the office's files name it: a bridge by its entry's `from`, a tools file by a worker that holds its tools, a rule by a team's heed or a list of deciders. A bridge is copied when the office holds a folder `bridges/<name>/` whose page begins as the shipped bridge's does; the copy's version is the newest its own Changed names. What a thing reaches is read from its definitions, never from its page. Its settings are a bridge's entry as shipped, and, of any other, each its definitions' input gives under `settings`, with its default and what it is for. **Use** drafts a bridge's entry as shipped in `office.yaml`, under its own name, or the first after it that is free, as `slack-2`. **Copy** drafts a bridge's machines, script and page into `bridges/<name>/`, for each entry that runs it, or a new one when none does, with each entry's `from` taken out; and a rule's files into the folder of each team that names it, with each line that names it naming the copy. A tools file is not copied, and neither is a rule no team names. Either opens a draft of the person's own, checked and put as any is.

A team's docs, each by its name with its size, and each opened whole, are seen by its people, the people of each team within it, whose workers read them, and the people who decide a change, and by nobody else; seeing a team's docs shows nothing else of how it is written.

**What the office defines** is shown in one view, worked out from the files each time: of each team whose workings a person sees, the tools files Bulldog ships, the scripts and the other machines its workers' tools run, and the models its workers send, each with those teams that use it; and each doc the person may open, with its team and the teams whose workers read it. Nothing is shown that the teams' own pages do not show them.

Who may say more is one rule, the same on the screens as through a bridge, and the requests area's. A sub-request is asked at no place, so nobody says more in it from an origin. A guest says more only at the origin, through the bridge, and never signs in. Seeing a sub-request from the request it was asked from gives no voice in it.

The top team's people also see a bridge that is failing, with what Corbie said; to anyone else it is running. A worker at work on something a person may not see is not shown at work to them.

## The API

JSON in and out, with the key in its cookie. `/api/me` gives the office's `zone`, in which the screens show every time. A request that adds an entry with files is sent as a form of several parts, the files and the text together.

| Request | Gives or does |
|---|---|
| `GET /api/me` | who is looking, with who they are on each bridge, the lists that name them and the teams whose decisions end at them; the office as everyone sees it, its runs, who decides a change and its bridges with their places, with each bridge's settings and the hosts and secrets its machines reach to the people who decide a change, with a team's heed's rule and its settings and whether it sets a disclosure to those who see how it is written, what waits on them and how much, the problems they may see, and, to the top team's people, which bridges are failing |
| `GET /api/requests` | the requests the one looking may see, newest first; by `team`, by `state`, queued among them, by the `bridge` and `place` they were asked at, those they `asked`, or those `waiting` on them; a request noted, every message in it noted by its heed, only when `noted` asks for them, with how many such there are; each with its state, a request that waits its turn with its place in the queue, or, for one from an issue, its source's status as last reported, with when, and whether its work is going or waits on someone; in part, 100 at a time unless a `limit` asks for more or fewer, and no more than 500, with `before` a request's number for those numbered below it, and the `before` to give for the next part when more are left |
| `GET /api/requests/{n}` | one request: its one status, a working one queued with its place in the queue while it waits its turn, an answered one stopped, noted or quiet; what was done, and what was read of each team the one looking is in; its thread in part, with `before` to reach further back, each entry by and put to whoever the office names now, a person's words on a bridge shown as theirs, a message its heed noted marked so, and an entry of a decision naming the decision; the teams it asked and the questions put to people in it; where its answer stands, waiting, said, not delivered with why, or quiet, and held to its team's people alone; its decisions as far as they see them: a held action or a change, what it is, how it stands and who ended it, to whoever sees what was done, and a disclosure to its team's people alone, each named as it is built when shown, the notes, the walk and an action's arguments to the people of the team whose decision it is and those asked it, a worker among them with the team it is of, and what a disclosure would let out, whom it is to, where it may in the end be read and the requests it goes through, to those asked it alone; and, to its team's people, its disclosure, whether its answer needs it and who decides it, and its runs, a heed's with what it said, each with the colleagues it called beneath it |
| `GET /api/teams/{team}` | one team: everyone sees it as in `/api/me`, and its people see the rest of it: its lists and its disclosure, each walked; its rules; its models with their secrets; its routines with when each comes round next and its status; each worker's model and its secret, what its file writes of how it runs and the files it reads, its tools, a flow's stages or a statechart's jobs, with what each does, what its job is, one of Bulldog's with the kit it comes in, what a script's or a machine's may reach, and, for one that waits for a yes, its deciders in the order the walk asks them, each a person, a rule or a worker, and a worker with the team it is of; the colleagues whose held jobs come to its ask; and its teams it may ask; its notes, each whole, with its size and how much of it a run is given, and the workers that may write them; and its docs, by name and size, to those who may open them |
| `GET /api/defined` | what the office defines, as the one looking may see it: the tools files, scripts and machines the tools of the teams they see run and the models they send, each with the teams that use it, and the docs they may open, each with its team and the teams that read it |
| `GET /api/manual`, `GET /api/manual/{page}` | Bulldog's pages of the manual, each by its name and its first line, and one page whole, to everyone signed in |
| `GET /api/shipped` | each thing Bulldog ships, as the one looking may see it: its page, what it reaches and its settings as shipped, whether it is used and where, and, to the people who decide a change, what changed in it since the version that ran before, the office's copies of it, and the drafts Use and Copy start |
| `GET /api/teams/{team}/docs/{doc}` | one doc of the team, whole, by its name, a path in its `docs/` folder without `.md`, to those who may open it |
| `GET /api/office/files`, `GET /api/office/file?path=` | the office's files, to the people who decide a change |
| `GET /api/history`, `GET /api/history/{id}` | every change to the office's files, newest first, and one change with each file before and after, to the people who decide a change |
| `GET /api/events` | a stream: `hello` when a screen connects, then `change` naming what changed, or `runs` when how many runs are at work or wait their turn, or which requests wait their turn, changes |
| `GET /api/runs/{run}` | one run, to those who may watch it: its kind, its team, its request, the worker or rule it is of, and how it stands; and, of a run of a lead or a routine's worker, what it was given, worked out when it is shown |
| `GET /api/runs/{run}/watch` | a stream of what one run does, as it does it |
| `GET /files/{id}` | a file on an entry |
| `POST /api/requests` | asks in the chat |
| `POST /api/requests/{n}/entries` | says more in a request, or answers a question in it |
| `POST /api/requests/{n}/decisions/{id}` | decides |
| `POST /api/requests/{n}/stop` | stops a request |
| `POST /api/teams/{team}/routines/now` | runs one of its routines now |
| `POST /api/office/check` | reads what a change would do, and writes nothing |
| `POST /api/office/put` | puts a change to be decided, as its check was shown |
| `POST /api/me/key` | replaces the one looking's key |

Open without a key:

| Request | Gives or does |
|---|---|
| `POST /signin` | asks for a one-time link, by name; answered the same whatever the name |
| `GET /signin/{link}` | the page of a one-time link, which uses nothing up |
| `POST /signin/{link}` | presses it: signs the person in, as `/?key=` does, and uses the link up |
| `POST /bridges/{name}` | a delivery to a bridge that hears, handed unread to a new run of its in machine, which checks with the secret it holds that it is the service's |

**A thing the one looking may not see is not there**: it is answered exactly as one that does not exist. Without a key, the answer says so. What the rules refuse is answered with a sentence a person can read, and so is a body that cannot be read or is too large, which is never taken in part. A failure is a failure, said as one, and is in the log.

**The streams show only what the one looking may see.** A `change` names a request they may see, as it or a run on it changes, what waits on them, or the office read again, and a screen fetches only what it names. A screen that connects, or falls behind, is sent `hello`, and fetches what it shows. A run's stream is Corbie's record of it, with every event in its place, so a screen that connects again goes on from where it was and nothing is shown twice.

**A file is served so that it can do nothing.** Only a picture or a PDF is shown, and anything else is downloaded; nothing put in is ever run as a page of the office.

The screens are in the program and load nothing from any other site. Besides them and the sign-in, only the address of a bridge that hears is open without a key.

## Interfaces

- **Office** is read from the folder the server readies, gives each worker's model with a one-word name filled in, what each of its tools may reach, each model's secret and when each routine comes round next, which the server shows, and reports its problems, which the server shows in the log and to those who may see them.
- **Requests** keeps its part of the store the server opens, says who sees each request and each layer of it, who may say more in it, and what waits on whom, says who stands as the people of a team with no people, and gives the request a routine opened that is still open; the server shows what it says.
- **Work** looks in Corbie for another server of the office before the server opens anything of it, is started by the server once it listens, labels every run with the office's identity the server holds, starts and renews the server's own run for it and otherwise leaves that run alone, stops a request when a person who may stop it asks, says which teams each worker may ask, gives a run as it keeps it to those who may watch it, with what a run of a lead or a routine's worker was given, worked out again from the request and the office, and gives how many runs are at work and which requests wait their turn, in the order they start, saying when either changes.
- **Decisions** is given a person's answer from the screens, as that person, and says that a worker asked to decide for a team runs as that team's; it builds each decision's line when it is shown, and gives the walk for a team and a key, and for a team's disclosure, by which the screens show who decides a tool, a list and a disclosure, and `check` lists who decides a tool.
- **Bridges** carry a one-time sign-in link to a person's direct place, one out run at a time, telling the server how it ended, give each bridge whose runs are failing, and are called at an address the server serves without a key.
- **Changes** is given a person's draft to check and put, and names the people who decide a change, to whom the server shows the office's files and the history; the server makes the repository of a folder that has none, and the office's first commit is the one commit Bulldog makes that is neither a decided change nor a team's note.
- **Corbie** is reached as a client, named as `corbie` itself is told.
