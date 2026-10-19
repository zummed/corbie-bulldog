# Changes

The files are the office, and they change in one way. A change is drafted, as the whole new text of each file it touches; checked, by reading the office as the change would make it; decided, by a person; written, every file or none; and kept, as a commit that says who, why and who decided. A worker drafts a change with the builder's tools, and a person drafts one on the screens; from the draft on, they go the same way. Writing a team's notes is the one write to the office's files that is not a decided change: the team's own workers write them directly when their definitions name the notes tool, and each write is still a commit that says who wrote it and why.

```
Sam, in the chat: "Nobody can tell me whether the washing can go out. Sort that out."
  → the steward hands it to the builder, its colleague
  → the builder reads the office's files and the manual
  → it drafts weather.yaml, a new worker, and tools/forecast.js, the script behind its one tool
  → it checks the draft: Corbie would refuse the definition, and says why. It mends it
  → it puts it: "A worker for the weather. You will need to set WEATHER_KEY."
Sam is shown why, every file whole, and what Bulldog reads the change to do:
      weather joins the house.
      weather's model is claude-haiku-5-5 (anthropic).
    it lets more by:
      weather may connect to api.weather.example.
      weather may send the secret WEATHER_KEY to api.weather.example.
  → Sam says yes on Telegram. The files are written, the change is a commit,
    and weather is a worker of the house from then on
```

## The builder

A builder is a worker whose definition names the builder's tools, a tools file Bulldog ships, beside whatever else it names. Nothing else marks it, and it may sit in any team. The tools are safe to hold, because all they lead to is a question to a person.

| Tool | What it does |
|---|---|
| `office_files()` | every file of the office, and what the draft does to each |
| `office_read(path)` | one file, as the draft has it or as it is |
| `office_write(path, text)` | puts the whole new text of a file into the draft, making the file if it is not there |
| `office_remove(path)` | has the draft remove a file |
| `office_check()` | reads the office as the draft would make it: every problem, and what the change says |
| `office_put(why)` | puts the draft to be decided, and waits for the decision |
| `manual(page)` | how an office is written, with whole examples of a worker, a rule and a bridge, and any page of Corbie's manual |

Each is a tool Bulldog ships: a run that waits on a ticket, which Bulldog answers. `office_read` reads any file of the office, every team's notes among them, since notes are files and are not sealed.

## A note

A worker writes one of its own team's notes with the notes tool, `write_notes(name, text, why)`, in the `tools/notes/notes.yaml` Bulldog ships, which a worker has only when its own definition names that file; a team that does not want its workers writing notes names it in none of them. Reading takes no tool, since every run Bulldog starts of a team's worker is given the team's notes, and which note a run may write is the work page's. Writing one needs no builder and waits for no yes: a note is what a team knows, not how the office works, and it is the one write to the office's files that is not a decided change. It is still a commit that says who and why: the note's whole new text is written as `notes/<name>.md` in its team's folder and committed at once, by the worker, with its why and the request it was written in, and the commit is the tool's answer.

A note is written and kept by the same steps as a change, without the decision: whole beside itself and moved into place, never over an edit by hand not yet committed, in a commit that holds that one file and no identity of a real person. It stands apart from the request's draft. Writing it neither adds to the draft nor puts or withdraws what the builder has drafted; a change drafted against the note's old text then finds the file no longer as it was drafted against, and is not written.

A note is not sealed. The builder reads it, whoever decides a change is shown it where a change touches it, and the people who decide a change see it in the history. A secret belongs in Corbie's secrets, and what a team read belongs in its runs, never in a note.

## The draft

A draft belongs to the request the builder is working on, and outlasts its runs: a change may take several, with questions to a person in between. For each file it touches it holds the whole new text, or that the file goes, and the text the file had when it was first drafted; a file drafted again after it changed in the folder is drafted anew, against the folder as it is then. Writing a file's present text back, or removing a file that only the draft made, takes that file out of the draft, so the draft holds only what changes.

A draft may hold any file of the office. It may not hold anything of Bulldog's own: the files Bulldog ships, its store, or the repository.

## Checking a change

A change is checked by reading the office as it would make it, as the office is always read: every team file, every person, and every worker, rule and bridge machine checked by Corbie, with every problem said together and each once. A change is refused for any problem the office does not have now. A problem the office already has is said beside, and does not stop a change that leaves it as it was, so one broken worker does not stop every change.

Some things are a person's to write, and a worker's draft that touches them is refused: the people of the office and the identities each is reached by, since how a person is reached is their entry in the files; the list of who decides a change to the office, the files of the deciders on it, and the `models` entry a worker on it takes its model from; and a bridge's in machine and the files it uses. An in machine says who a person is, and so who has said yes. A person's draft that changes how someone is reached, their own entry included, is decided as any change is, since changing one's own identity on a bridge is what someone taking over an account would do.

## What a change says

Bulldog says what a change does in its own words, read from the files as they would be and from nothing whoever drafted it said. It says each team that is new, gone or moved, and each change to a team's `about`, people, lead, deciders, disclosure check, routines, `models` and notes, and to the office's `zone` and `runs`; each person new or gone, and each identity given, changed or taken away; each worker new, gone or moved, and each change to its model, as Bulldog fills it in where it is named by one word, to what it may reach, to the secrets it holds and where they may go, to its tools, and to which of them wait for a yes, so that a change to a team's `models` is said for every worker whose model it changes; each rule new, gone or changed, and, for a rule that is a definition, what it may reach and the secrets it holds; and each bridge new or gone, and each change to its machines or its settings, among them each place it listens in that is new, gone or changed, with the team asked there and whether guests may ask there. Every file in the change is in at least one sentence.

**What lets more by is set apart.** A change lets more by where someone may do or see more than they could, or something that waited for a yes no longer does: a permission or a secret gained, by a worker, a rule or a bridge's machine; a model, written out or filled in from a name, at a new address or naming a new secret; a tool that no longer waits; someone who now sees what a team reads, named in it or standing as the people of a team with none; a decider taken off a list or something put before its first person; a disclosure check taken away. A rule whose text, or a file it uses, changes, where it decides anything, is always set apart, because Bulldog cannot read what a rule decides; so is a bridge whose in or out machine or whose settings change, because Bulldog reads only the places in a bridge's settings, and not what the rest lets in. What narrows is said, and not set apart.

What a definition may reach, and which of its tools wait for a yes, is Corbie's reading of it. Bulldog takes it from the definition as Corbie read it, and its jobs as Corbie places them, never from a reading of its own.

A worker whose definition goes from one team's folder while one of the same name comes into another's has moved, and is said once, as a move. It takes with it, in the same change, the files only its definition uses, whoever drafted the move, but for one its moved definition still names where it is. None of them is written over a file already there.

## Who decides

A change is a decision of the top team, asked of its `change` list; of its `everything` when it writes none; and of its people when it writes neither. The people who decide a change are the people the `change` list names; where it names no person, those its `everything` names; where neither names a person, the top team's people. They alone put a change from the screens and read the history.

```yaml
# office.yaml
decide:
  everything: [sam]
  change: [./no-secrets.js, sam]
```

**Only a person's yes writes a change.** The list is walked in order. A rule or a worker on it, of any team, may refuse the change, which ends it, or pass it on with a note; it never approves one, and a yes from either is taken as a pass, its note carried on to whoever comes next. A list that names no person, when every rule and worker on it has passed, goes on to the top team's `everything`, or its people where it writes none, walked the same way, which always names a person. So every change that is not refused on the way ends with a person, who says yes or refuses. A change whose walk, as it was put, comes to nobody still in the office, because those on it have left, is refused by Bulldog, with a note saying why, and whoever drafted it is told as for any refusal; Bulldog never says yes to a change. This holds whatever a yes from a rule or a worker means for any other decision: otherwise a worker could rewrite a rule, and the rule approve the rewrite. How a person is asked, and that their yes is taken only from their checked identity, is the decisions page's.

A change is judged by the office as it is, never as the change would make it: its lists, and the rules and workers on them, are those in the folder now. A change that rewrites the rule judging it is judged by the rule it replaces.

A change that touches a file with an edit by hand not yet committed is refused when it is put, before anyone is asked, since it could never be written.

Whoever decides is shown why, in the words of whoever drafted it; what the change says, with what lets more by set apart; and every file whole, beside what it replaces. What is written on a yes is exactly what was shown: a change's files are fixed when it is put, and a draft changed afterwards is put again, in place of the change that waited, which is withdrawn.

Refusing writes nothing. Whoever drafted it is given the note, and the draft is kept, to be changed and put again. A decision that comes after the run that put the change has ended, or after `office_put` has stopped waiting, is acted on all the same: its outcome is an entry in the request, which starts the request's lead again, as any late answer does.

## Writing it

On a yes, the office is read with the change once more, since the folder may have moved on. Nothing is written if the change would now leave a problem; if any file differs from the text it was drafted against; or if any file it touches has an edit by hand that is not yet committed, which would otherwise be committed as the work of whoever drafted the change. Every file is written, or none is: each is written whole beside itself and moved into place, and if one fails, every file already written is put back as it was.

The files are then committed. A commit that cannot be made puts the files back, so that nothing is ever written that is not kept. Anything that stops a change being written ends its decision as not written, with why, told to whoever decided and in the request, and the draft is kept. Once it is written, the draft is cleared, the office is read again, and everything begun after is begun in the office as it now is. A worker the change took away has its runs cancelled.

## A person's change

A person who decides a change drafts one on the screens: whole files written or removed, one field of a file set, or a worker moved to another team or taken away. As they go they are shown what a builder's check shows: every problem, what the change says with what lets more by set apart, and every file as it would be. Their screens hold the draft, and nothing is written until it is put.

Moving a worker takes its definition and the files only its definition uses to the new team's folder, and points what its definition names to files that stay where they are. Taking a worker away removes its definition and the files only it uses; what another uses stays.

A bridge Bulldog ships is set by a change that writes its entry in the `bridges` of `office.yaml`, with the `from` that names its folder and the settings its page gives, as its `settings.yaml` holds them; nothing of its folder is copied into the office, and it runs as Bulldog ships it. The Telegram bridge's entry is `telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }`: it hears people's direct messages to the bot. A bridge's folder of the office's own is drafted, changed and written as any files are; its in machine is a person's to write, so a worker's draft that holds one is refused.

A person who decides a change puts their own. It is the same decision as a worker's, and goes to the deciders before it reaches a person: every rule and worker the walk asks before its first person is asked first, and may refuse it; when they have passed, the person who made it is the one who said yes. Where the walk asks nobody before its first person, as in the house, it is written as it is put, with their yes. Only the people who decide a change put one from the screens. Anyone else asks for one in a request, a builder drafts it, and it is decided as any change is.

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

A commit is by the worker or person who made the change, by name and with no address or identity of a real person, and holds the change's files and no others. Its first line is the first line of why, and the rest of why follows; then what Bulldog read the change to do, with what lets more by under a heading of its own; then who decided, their note, and the request it was made in. A person's own change names them as who decided. A note's commit names nobody as having decided.

The folder may be the repository, or a folder within a larger one, and then the history is that of its own files. On the screens, the history is seen by the people who decide a change, and by nobody else: each change with who, why and who decided, each note with who wrote it and why, and each file as it was and as it is.

## Edits by hand

What a person changes in the folder by hand is theirs to commit, and Bulldog never commits it. The one exception is the office's first commit, when the server makes a repository for an office that has none and commits what is there. Until they do, the screens say which files are not as the last change left them, and no change or note is written over them. A hand edit that leaves the office with a problem is mended by hand, or committed and then mended by a change; how the office is read meanwhile is the office page's.

## Interfaces

- **Office** reads the folder, with a change laid over it, and gives every problem once, each worker's model as filled in and each place a bridge's settings list; it says which files are the office's, which files only one worker's definition names, and reads the top team's `change`, `everything` and people.
- **Decisions** runs each rule and worker on a change's walk and asks its people, as for any decision, and accepts a person's yes only from their checked identity; how a change's list is walked, that only a person approves one, and that it is judged by the office as it is, are said here. A person's own change is marked so on what is put (`Person`), and its walk holds only the rules and workers before its first person: Decisions walks it in order, a refusal from any of them ends it refused (`Decided` with that refusal and its note), and when every one has passed, Decisions calls `Decided(id, Decision{By: put.By, Kind: person, Called: …, Yes: true})`, the yes of the person who put it. Each decision carries what its decider is called, a person's name as the office writes it, so that whoever drafted the change is told it in the words Bulldog speaks to people: "with the yes of Sam", "refused by Sam". A change nobody on its walk is left to decide, Decisions ends with `Decided(id, Decision{By: "Bulldog", Kind: bulldog})`, Bulldog's refusal with its note, which Changes takes from nobody else not on the walk; a yes of that kind is not taken. A person's own change whose walk is empty is written as it is put, and never reaches Decisions.
- **Work** gives the notes tool, `write_notes`, to a worker whose definition names `tools/notes/notes.yaml`, gives every run it starts of a team's worker the team's notes, and says which note a run may write; it hands each call to the builder's tools, and each note's write, to Changes, runs the builder's tools as runs that wait on tickets, gives the decision to the run that put the change or to its request once that run has ended, starting the lead again, and cancels the runs of a worker a written change takes away.
- **Requests** keeps each draft with its request, and each change put and its decision in what was done.
- **Bridges** reach the people who decide a change; Bulldog ships the bridges a change sets by their entry, with the settings each is set with, an in machine is a person's to write, a bridge's folder is drafted, checked and written as any files are, and the shape of a bridge's settings and its places is Bridges'.
- **Server** gives the screens a person's draft, its check and its putting, and the history, to the people who decide a change; writes the builder's tools file at every start as the program has it; and makes the repository of a folder that has none, with the office's first commit (`Begin`), or, for an office it has only now written inside another repository, one of the office's own (`BeginOwn`). It reads here which of a worker's tools wait for a yes (`Waits`), to show who decides each. What this area does not allow, of an edit, a change put or the history, is a `Refusal`, said in a sentence for whoever asked; a change the history does not hold is `ErrNotThere`; anything else is a failure. The server answers them as such, and asks no question of its own that this area answers.
