# Changes

The files are the office, and they change in one way. A change is drafted, as the whole new text of each file it touches; checked, by reading the office as the change would make it; decided, where its tool waits for a yes; written, every file or none; and kept, as a commit that says who, why and who decided. A worker writes with the tools Bulldog ships, and a person drafts a change on the screens; from the draft on, they go the same way.

There is one idea in the tools that write: a worker's Bulldog tools reach its own team's files and those of the teams within it. Whether a write waits for a yes is configuration, never Bulldog's: it is Corbie's `approve: true` on the tool's job in its tools file. As shipped, `write_note` is written at once, and `write_doc` and `put_draft` wait for a yes. An office changes that by copying the tools file into its own folder, or by writing the tool in a worker's `tools:`, and Bulldog honours whatever the file says: a write whose job is not held is written when Bulldog answers its call, and one that is held is a held action, decided as any other.

```
Sam, in the chat: "Nobody can tell me whether the washing can go out. Sort that out."
  → the steward hands it to the builder, its colleague
  → the builder reads the office's files and the manual
  → it drafts weather.yaml, a new worker, and tools/forecast.js, the script behind its one tool
  → it checks the draft: Corbie would refuse the definition, and says why. It mends it
  → it puts it: "A worker for the weather. You will need to set WEATHER_KEY."
    put_draft waits for a yes, as Bulldog ships it, so Corbie holds it, and
    the house decides it: the files are the house's, and its everything is Sam
Sam is shown why, every file whole, and what Bulldog reads the change to do:
      weather joins the house.
      weather's model is claude-haiku-5-5 (anthropic).
    it lets more by:
      weather may connect to api.weather.example.
      weather may send the secret WEATHER_KEY to api.weather.example.
  → Sam says yes on Telegram. put_draft runs, Bulldog writes the files, the change
    is a commit, and weather is a worker of the house from then on
```

## The tools that write

Bulldog ships its tools in small files, one folder of `.bulldog/tools/` for each purpose, and a worker names in its `tools_file:` exactly the files whose tools it needs. Three folders hold tools that write the office's files:

| File | Its tools | Reaches | Waits, as shipped |
|---|---|---|---|
| `tools/notes/notes.yaml` | `write_note`, and its team's notes given at each run's start | its own team's `notes/` | no |
| `tools/docs/read.yaml` | `list_docs`, `read_doc`, `search_docs` | its own team's docs and those of each team around it | |
| `tools/docs/write.yaml` | `write_doc` | its own team's `docs/` | `write_doc` |
| `tools/office/read.yaml` | `list_files`, `read_file`, `search_files`, `manual` | the files of its own team's folder and of the teams within it; the top team's, the whole office | |
| `tools/office/write.yaml` | `write_file`, `remove_file`, `check_draft`, `put_draft` | the same | `put_draft` |

A worker that names the notes tools is given its team's notes at the start of each run, and has `write_note`; one that names them not is given no notes. A worker that names the docs tools that read reads docs when it asks; one that also names those that write has `write_doc(name, text, why)`, which writes or replaces one doc in its own team's `docs/`, so a worker that finds a doc wrong or out of date proposes the fix. A worker that writes reads first, so one that names a `write.yaml` names the `read.yaml` beside it; nothing enforces that, and a worker that names only the writes drafts blind.

## The office tools

A worker that names the office tools reads, and with those that write drafts, the files of its own team's folder and the folders of the teams within it, beside whatever else it names. Nothing else marks it, and it may sit in any team: one of the garden drafts the garden's files and nothing of the house's, and one of the house drafts anything of the office. A path outside that reach is refused by every one of its tools, saying why.

| Tool | File | What it does |
|---|---|---|
| `list_files()` | read | every file it reaches, and what the draft does to each |
| `read_file(path)` | read | one file, as the draft has it or as it is |
| `search_files(words)` | read | the lines of the files it reaches that hold the words as written, in any case, each as `path:line: text`, as the draft has them or as they are, up to 20,000 characters, and never of Bulldog's own in `.bulldog/` or the repository |
| `manual(page)` | read | how an office is written, with whole examples of a worker, its tool keeping the service's reason, a rule and its whole input, rules in a chain, a heed and its whole input, and a bridge, which listens or hears; and any page of Corbie's manual |
| `write_file(path, text)` | write | puts the whole new text of a file into the draft, making the file if it is not there |
| `remove_file(path)` | write | has the draft remove a file |
| `check_draft()` | write | reads the office as the draft would make it: every problem, and what the change says |
| `put_draft(why)` | write | puts the draft: writes it, once any yes its job waits for is given |

Each is a tool Bulldog ships: a run that waits on a ticket, which Bulldog answers. `read_file` and `search_files` read any file in reach, the notes of each team in it among them, since notes are files and are not sealed.

## A note and a doc

A worker writes one of its own team's notes with `write_note(name, text, why)`, of the notes tools, which a worker has only when its own definition names that file; a team that does not want its workers writing notes names it in none of them. Reading takes no tool, since each run of a worker that names the file is given the team's notes, and which note a run may write is the work page's. As shipped it waits for no yes: a note is what a team knows, not how the office works. An office that wants notes decided marks `write_note` with `approve: true` in a copy of the file, and then it is held and decided as any held action is, by its team's `write_note` list or else its `everything`. Either way it is a commit that says who and why: the note's whole new text is written as `notes/<name>.md` in its team's folder and committed, by the worker, with its why and the request it was written in, and the commit is the tool's answer.

A worker writes one of its own team's docs with `write_doc(name, text, why)`, of the docs tools: the whole new text of `docs/<name>.md` in its own team's folder, a name being a doc's name as `list_docs` gives it, folders and all, as `rooms/kitchen`. As shipped it waits for a yes, so a doc a worker finds wrong is mended only once its team's deciders say yes. It is a change of one file: held, its deciders are shown the doc whole beside what it replaces, and once it runs it is written as a change is, with who decided.

A note, and a doc, is written and kept by the same steps as a change: whole beside itself and moved into place, never over an edit by hand not yet committed, in a commit that holds that one file and no identity of a real person. It stands apart from the request's draft. Writing it neither adds to the draft nor puts what the worker has drafted; a change drafted against its old text then finds the file no longer as it was drafted against, and is not written.

A note is not sealed. A worker that reads the office's files reads it, whoever decides a change is shown it where a change touches it, and the people who decide a change see it in the history. A secret belongs in Corbie's secrets, and what a team read belongs in its runs, never in a note.

## The draft

A draft belongs to the request the worker is working on, and outlasts its runs: a change may take several, with questions to a person in between. For each file it touches it holds the whole new text, or that the file goes, and the text the file had when it was first drafted; a file drafted again after it changed in the folder is drafted anew, against the folder as it is then. Writing a file's present text back, or removing a file that only the draft made, takes that file out of the draft, so the draft holds only what changes.

A worker's draft may hold any file in its reach; a person's, any file of the office. Neither may hold anything of Bulldog's own: the files Bulldog ships, its store, or the repository.

## Checking a change

A change is checked by reading the office as it would make it, as the office is always read: every team file, every person, and every worker, rule and bridge machine checked by Corbie, with every problem said together and each once. A change is refused for any problem the office does not have now. A problem the office already has is said beside, and does not stop a change that leaves it as it was, so one broken worker does not stop every change.

Some things are a person's to write, and a worker's draft that touches them is refused: the people of the office and the identities each is reached by, since how a person is reached is their entry in the files; the top team's `put_draft` list and its `everything`, the files of the deciders on them, and the `models` entry a worker on it takes its model from; a bridge's in machine and the files it uses; and which bridges the office has, each one's `from`, and every setting of each but its places. An in machine says who a person is, and so who has said yes; a bridge's entry says what it runs, where it listens and whom it signs in as. A worker's change to a bridge's places is drafted as any change, and set apart for whoever decides it. A person's draft that changes how someone is reached, their own entry included, is decided as any change is, since changing one's own identity on a bridge is what someone taking over an account would do.

## What a change says

Bulldog says what a change does in its own words, read from the files as they would be and from nothing whoever drafted it said. It says each team that is new, gone or moved, and each change to a team's `about`, people, lead, heed, deciders, disclosure check, routines, `models` and notes, and to the office's `zone` and `runs`; each person new or gone, and each identity given, changed or taken away; each worker new, gone or moved, and each change to its model, as Bulldog fills it in where it is named by one word, to what it may reach, to the secrets it holds and where they may go, to its tools, a flow's stages or a statechart's jobs, and to which of them wait for a yes, so that a change to a team's `models` is said for every worker whose model it changes; each rule new, gone or changed, and, for a rule that is a definition, what it may reach and the secrets it holds; and each bridge new or gone, and each change to its machines or its settings, among them each place it listens in that is new, gone or changed, with the team asked there and whether guests may ask there. Every file in the change is in at least one sentence.

**What lets more by is set apart.** A change lets more by where someone may do or see more than they could, or something that waited for a yes no longer does: a permission or a secret gained, by a worker, a rule or a bridge's machine; a model, written out or filled in from a name, at a new address or naming a new secret; a tool, stage or job that no longer waits; someone who now sees what a team reads, named in it or standing as the people of a team with none; a decider taken off a list, something put before its first person, or a rule or worker on it that no longer advises; a disclosure check taken away. A rule whose text, or a file it uses, changes, where it decides anything, is always set apart, because Bulldog cannot read what a rule decides; so is a bridge whose in or out machine or whose settings change, because Bulldog reads only the places in a bridge's settings, and not what the rest lets in. What narrows is said, and not set apart.

What a definition may reach, and which of its tools, stages or jobs wait for a yes, is Corbie's reading of it. Bulldog takes it from the definition as Corbie read it, and its jobs as Corbie places them, never from a reading of its own.

A worker whose definition goes from one team's folder while one of the same name comes into another's has moved, and is said once, as a move. It takes with it, in the same change, the files only its definition uses, whoever drafted the move, but for one its moved definition still names where it is. None of them is written over a file already there.

## Whose decision a write is

A change is no decision of its own kind. A held `put_draft` or `write_doc` is a held action: Corbie holds its job, gives it to the nearest ask above it, which is the ask Bulldog ships, and Bulldog asks a team's deciders by its list named for the tool, or else its `everything`, as for any held action. Which team's is read from the files the write touches: the innermost team whose folder holds every one of them. One rule keeps that safe: a write that touches a team file, a `team.yaml`, is decided by the team around that team, and a write to `office.yaml` by the top team itself. So a new worker drafted in the garden is the garden's to decide, its change to the garden's team file is the house's, and a change that touches both is the house's.

```yaml
# office.yaml
decide:
  everything: [sam]
  put_draft: [reviewer: advise, sam]   # the reviewer may refuse; its yes goes to Sam as a recommendation
  write_doc: [./small-docs.js, sam]
```

A rule's or a worker's yes approves a write as it approves any action; a team that wants a person to decide its writes puts one last on the list, and one that wants a check first marks it to advise. Which writes wait is the tools file's alone: Bulldog never decides of itself that a write waits, nor that one does not.

When its job is held, the write is fixed: the request's draft is checked and its files, what it says and what it lets more by are kept with the decision, or, for a doc, the one file. A draft that is empty, says no why, has a problem the office has not, or touches a file with an edit by hand not yet committed is not asked of anyone: Bulldog refuses it, with what the check says as its note, and Corbie tells the worker, which mends it and puts it again. Whoever decides is shown why, in the words of whoever drafted it; what the change says, with what lets more by set apart; and every file whole, beside what it replaces, on the screens and in the direct message a bridge carries.

A write is judged by the office as it is, never as the write would make it: its lists, and the rules and workers on them, are those in the folder now. A change that rewrites the rule judging it is judged by the rule it replaces.

Refusing writes nothing: Corbie tells the worker it was denied, with the note, and the draft is kept, to be changed and put again. On a yes the job runs, and Bulldog answers its call by writing what was fixed, exactly what was shown, and the tool's result says it was written, with who said yes and the commit, or why it was not. A run that ends while its write waits withdraws the decision, as for any held action.

## Writing it

A write is written when Bulldog answers its call: at once where its job is not held, and once it runs after a yes where it is. The office is read with the change once more, since the folder may have moved on. Nothing is written if the change would now leave a problem; if any file differs from the text it was drafted against; or if any file it touches has an edit by hand that is not yet committed, which would otherwise be committed as the work of whoever drafted the change. Every file is written, or none is: each is written whole beside itself and moved into place, and if one fails, every file already written is put back as it was.

The files are then committed. A commit that cannot be made puts the files back, so that nothing is ever written that is not kept. Anything that stops a change being written is said, with why, as the tool's result and in the request, and the draft is kept. Once it is written, the draft is cleared, the office is read again, and everything begun after is begun in the office as it now is. A worker the change took away has its runs cancelled.

## A person's change

A person who decides a change drafts one on the screens: whole files written or removed, one field of a file set, or a worker moved to another team or taken away. As they go they are shown what a worker's check shows: every problem, what the change says with what lets more by set apart, and every file as it would be. Their screens hold the draft, and nothing is written until it is put.

Moving a worker takes its definition and the files only its definition uses to the new team's folder, and points what its definition names to files that stay where they are. Taking a worker away removes its definition and the files only it uses; what another uses stays.

A bridge Bulldog ships is set by a person's change that writes its entry in the `bridges` of `office.yaml`, with the `from` that names its folder and the settings its page gives, as its `settings.yaml` holds them; nothing of its folder is copied into the office, and it runs as Bulldog ships it. The Telegram bridge's entry is `telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }`: it hears people's direct messages to the bot. A bridge's folder of the office's own is drafted, changed and written as any files are; its in machine is a person's to write, as a bridge's entry is, so a worker's draft that holds either is refused.

On the screens, a person may start such a change from what Bulldog ships. **Use** drafts a shipped bridge's entry as it is shipped, under its own name, or the first after it that is free when that is taken. **Copy** drafts a copy into the office: a bridge's machines, script and page into `bridges/<name>/`, with its entry's `from` taken out in the same change, and a rule's files into the folder of each team that names it, with each line that names it naming the copy. A copy is the office's own, and a later Bulldog does not change it. A tools file is not copied: a worker narrows a tool by writing it over the call Bulldog ships. Either is a draft like any other, the person's to change, check and put.

A person who decides a change puts their own, and it is decided as if they had called `put_draft`: by the team whose folder holds its files, as above, from its `put_draft` list or else its `everything`. Every rule and worker that walk asks before its first people is asked first, and may refuse it; when they have referred it, or recommended yes, it is written with the yes of the person who made it where they are among those people, and is asked of those people where they are not. Where the walk asks nobody before people the person is among, as in the house, it is written as it is put, with their yes. Only the people who decide a change put one from the screens. Anyone else asks for one in a request, a worker drafts it, and it is decided as any write is.

## Rewriting part of a file

Where Bulldog changes part of a file a person wrote, setting a field or moving a worker, only the lines that change are changed. Comments, blank lines, how far the file is set in, how its lines end and how it ends, and how each line is set out are as they were. A rewrite is used only when it reads as the same value as the file written plainly; where it does not, the plain text is used, and the person is told the layout was not kept. A file that marks where its documents begin or end is never rewritten in part.

## The history

The office's folder is in a git repository. Bulldog reads and writes it with a library, runs no git program, and never pushes or pulls: where the repository goes is its people's. The history is read with anything that reads git.

```
commit 4f1c…    builder    4 Oct 14:02
    A worker for the weather

    Nobody could say whether the washing could go out, and three requests
    about it this week went unanswered.

    What it does:
    - weather joins the house.

    What it lets more by:
    - weather may connect to api.weather.example.

    Decided-by: sam
    Note: Good. I have set the key.
    Request: 12

commit 9a2e…    steward    6 Oct 09:15
    Breaker 4 is the kitchen

    What it does:
    - the house's note fuses is changed.

    Request: 14
```

A commit is by the worker or person who made the change, by name and with no address or identity of a real person, and holds the change's files and no others. Its first line is the first line of why, and the rest of why follows; then what Bulldog read the change to do, with what lets more by under a heading of its own; then who decided, their note, and the request it was made in. A person's own change names them as who decided. A write whose job was not held names nobody as having decided.

The folder may be the repository, or a folder within a larger one, and then the history is that of its own files. On the screens, the history is seen by the people who decide a change, and by nobody else: each change with who, why and who decided, each note with who wrote it and why, and each file as it was and as it is.

## Edits by hand

What a person changes in the folder by hand is theirs to commit, and Bulldog never commits it. The one exception is the office's first commit, when the server makes a repository for an office that has none and commits what is there. Until they do, the screens say which files are not as the last change left them, and no change or note is written over them. A hand edit that leaves the office with a problem is mended by hand, or committed and then mended by a change; how the office is read meanwhile is the office page's.

## Interfaces

- **Office** reads the folder, with a change laid over it, and gives every problem once, each worker's model as filled in, what each worker runs by name with whether it waits for a yes, and each place a bridge's settings list; it says which files are the office's, which files only one worker's definition names, each team's folder and team file, and reads the top team's `put_draft`, `everything` and people.
- **Decisions** takes a held `put_draft` or `write_doc` as it takes any held job, and asks Changes to fix it first (`Hold`), which gives the change fixed, or Bulldog's refusal of one that cannot be put, and the team whose decision it is (`Whose`); it walks the team's list for the tool, accepts a person's yes only from their checked identity, and tells Changes how the write was decided (`Decided`), so the write that runs after names who said yes. A person's own change is marked so on what is put (`Person`): Decisions walks the same team's list, and once every rule and worker before its first people has referred it or recommended yes, calls `Decided(id, Decision{By: put.By, Kind: person, Called: …, Yes: true})`, the yes of the person who put it, where they are among those people, and asks those people where they are not. Each decision carries what its decider is called. Every own change reaches Decisions, which ends it with the person's yes at once where the walk reaches people that include them.
- **Work** gives the notes tools' `write_note` to a worker whose definition names `tools/notes/notes.yaml`, gives each run of such a worker its team's notes, and says which note a run may write; it hands each call of the office tools and of `write_doc`, and each note's write, to Changes, runs them as runs that wait on tickets, and cancels the runs of a worker a written change takes away.
- **Requests** keeps each draft with its request, and each change put and its decision in what was done.
- **Bridges** reach the people who decide a change; Bulldog ships the bridges a change sets by their entry, with the settings each is set with, a bridge's entry, its `from` and its settings but its places, and an in machine, are a person's to write, a bridge's folder is drafted, checked and written as any files are, and the shape of a bridge's settings and its places is Bridges'.
- **Server** gives the screens a person's draft, its check and its putting, and the history, to the people who decide a change, and shows a held `put_draft` or `write_doc` as the action it is, with its files whole; writes the tools files Bulldog ships at every start as the program has it; and makes the repository of a folder that has none, with the office's first commit (`Begin`), or, for an office it has only now written inside another repository, one of the office's own (`BeginOwn`). What this area does not allow, of an edit, a change put or the history, is a `Refusal`, said in a sentence for whoever asked; a change the history does not hold is `ErrNotThere`; anything else is a failure. The server answers them as such, and asks no question of its own that this area answers.
