# Changes: rules

The detailed rules behind the [Changes overview](../changes.md). Requirements are `R-changes-nn`, constraints `C-changes-nn`. A "rule" here is a decider in its team's folder or among those Bulldog ships, a bare script or a definition that holds its own permissions, as everywhere in Bulldog; this page is a rules page in the other sense. A held write is decided by the team whose decision it is (R-changes-17), by its `decide` list named for the tool, or else by its `everything`. "The people who decide a change" are the people the top team's `put_draft` list names; where it names no person, those its `everything` names; where neither names a person, the top team's people (R-changes-38). A worker's reach is the files of its own team's folder and of the folders of the teams within it; the top team's worker reaches every file of the office.

## The tools that write

**R-changes-40** Bulldog ships its tools in small files, one folder of `.bulldog/tools/` for each purpose, and a worker names exactly the files whose tools it needs. Those that read or write the office's files are: `tools/notes/notes.yaml`, with `write_note`; `tools/docs/read.yaml`, with `list_docs`, `read_doc` and `search_docs`, and `tools/docs/write.yaml`, with `write_doc`; and `tools/office/read.yaml`, with `list_files`, `read_file`, `search_files` and `manual`, and `tools/office/write.yaml`, with `write_file`, `remove_file`, `check_draft` and `put_draft`. A worker that writes names the `read.yaml` beside the `write.yaml`, as each page says; nothing enforces it. Whether one of their writes waits for a yes is Corbie's `approve: true` on the tool's job in the tools file a worker names, and nothing else: as shipped, `write_note` has no `approve`, and `write_doc` and `put_draft` have `approve: true`.
*Accept:* every tools file Bulldog ships, read as Corbie reads it, holds `approve: true` on the jobs of `write_doc` and `put_draft` and on no other.

**C-changes-51** A write whose job is not held is written when Bulldog answers its call, and one whose job is held waits for its decision and is written when its job then runs, by what the tools file the worker names says alone: an office's copy of a tools file, or a tool written in a worker's `tools:`, that drops `approve` from `put_draft` or `write_doc` has it written at once, and one that adds it to `write_note` has the note held and decided as any held job. Bulldog never decides by itself that a write waits, or that one does not. Whether a write needs a yes is the office's configuration, and Bulldog logic beside it would be a second answer to the same question. *Test:* `changes_a_write_waits_by_its_tools_file_alone`, which holds that the house's builder's `put_draft` is held and written only after Sam's yes with `tools/office/write.yaml` as shipped, and written at once, with no decision, when the builder names an office copy of it without `approve`; and that the steward's `write_note` is written at once as shipped, and held for the house's `write_note` list when its copy of `tools/notes/notes.yaml` marks it.

**R-changes-41** `write_doc(name, text, why)` writes the whole new text of one doc of the calling worker's own team, `docs/<name>.md` in its folder, a name being a doc's name as `list_docs` gives it, with or without `.md`, folders and all. Held, it is a change of that one file, decided by the worker's team (R-changes-17) and shown as any change is (R-changes-18); written, it is a commit by the worker, with its why, who decided where anyone did, and the request. A name that climbs out of `docs/`, is absolute, or has a part beginning with a dot is refused; and a worker writes only its own team's docs, never those of a team within or around it.
*Accept:* the house's steward, naming the docs tools as shipped, writes `rooms/kitchen` with "The fuse is 5." and a why: it is held, the house's `everything` asks Sam, who is shown `docs/rooms/kitchen.md` beside its old text, and on Sam's yes the file holds the new text, in a commit by `steward` decided by `sam`. The gardener writes `boiler`: it is the garden's `garden/docs/boiler.md`. `../notes/fuses`, `/etc/x` and `.hidden` are each refused, and nothing is held.

## The office tools

**R-changes-01** A worker whose definition names `tools/office/read.yaml`, which Bulldog ships, is given `list_files`, `read_file`, `search_files` and `manual`, and one that names `tools/office/write.yaml` is given `write_file`, `remove_file`, `check_draft` and `put_draft`, and Bulldog answers each. Nothing but naming the files gives a worker the office tools.
*Accept:* a worker whose definition names both files calls each of the eight in one run, and each gives back an answer. A worker in the same team whose definition names neither is offered none of them. A worker called `builder` that names neither holds none of them.

**R-changes-02** `list_files` lists every file in the worker's reach, and says for each whether the draft makes it new, changes it, removes it, or leaves it as it is.
*Accept:* with a draft that writes one new file, changes one and removes one, the house's builder's list holds every file of the office, those three marked new, changed and gone, and the rest as they are; a worker of the garden lists the garden's files alone.

**R-changes-03** `read_file` gives a file's text as the draft has it where the draft holds it, and as it is in the folder otherwise; a file in neither is said not to be there, and one the draft removes is said to go.
*Accept:* reading a drafted file gives the draft's text; reading an untouched file gives the folder's; reading a path that is in neither gives an answer saying there is no such file; reading a removed file says the draft removes it.

**R-changes-42** `search_files(words)` gives the lines of the files in the worker's reach that hold the words as written, in any case, each as `path:line: text`, the files as the draft has them where it holds them and as they are in the folder otherwise, up to 20,000 characters, saying how many more lines hold them; never a line of Bulldog's own in `.bulldog/`, of the repository, or of a file that is not text.
*Accept:* with the house's builder's draft adding a line `# Lead: drafted` to `clerk.yaml`, its search for `LEAD:` finds `office.yaml`'s `lead: steward`, `garden/team.yaml:5: lead: gardener` and that line of the draft; its search for `machine` finds no file of `.bulldog/`; the garden's worker that names the office tools finds `garden/team.yaml` and nothing of the house's files; a search of no words searches nothing and says so.

**R-changes-04** `write_file` puts the whole new text of one file into the draft, making the file if it is not there.
*Accept:* writing a path that is not in the folder puts it in the draft as new, and `read_file` then gives that text; writing an existing path puts it in as changed.

**R-changes-05** `remove_file` has the draft remove a file of the folder.
*Accept:* removing an existing file marks it gone in `list_files`, and `check_draft` says it goes.

**R-changes-06** `check_draft` reads the office as the draft would make it and gives back every problem, and what the change says with what lets more by set apart, and puts nothing to be decided.
*Accept:* a draft with a team file that names a person who is not in the office, and a worker Corbie would refuse, gets both problems in one answer, with where each is; a draft with no problems gets the change's sentences, the ones that let more by apart; in neither case is a decision made.

**R-changes-07** `put_draft(why)` writes the draft when Bulldog answers its call, and its result is the outcome: written, with who said yes where its job was held, and the commit; or not written, with why. Held, it is refused before it runs as any held job is, and Corbie tells the worker it was denied, with the note. A decider is named by what they are called, as everywhere Bulldog speaks to people.
*Accept:* the house's builder puts a draft, held as `tools/office/write.yaml` ships it; Sam says yes, and the builder's run is given "written, with the yes of Sam" and the commit's id; on a second draft Sam refuses with a note, and the builder's model is told the job was denied, with the note; on a third, the folder is changed by hand and committed before the yes, and the run is given "not written" with the file that changed. With that file copied without `approve`, a put draft is written at once, and the result says "written" and gives the commit, with nobody's yes.

**R-changes-08** `manual(page)` gives Bulldog's page on how an office is written, with a whole example of a worker, a rule, rules in a chain, a heed and a bridge, and the whole input a rule and a heed are given, or any page of Corbie's manual by its name.
*Accept:* `manual("office")` gives Bulldog's page, which holds the weather worker, the small-orders rule, a second rule reading the first one's referral by its name, a heed with its settings and the Slack bridge's out machine; `manual` of a Corbie page's name gives that page's text as `corbie manual` prints it; a name that is neither is said not to be a page, with the names that are. Bulldog's pages are pages of Corbie's manual, so `corbie bulldog manual office` prints the same page.

**C-changes-02** Each of the office tools is a run that waits on a ticket, which Bulldog answers; nothing in Corbie calls Bulldog to answer it. A tool that needs the office is a run that waits. *Test:* `changes_office_tools_are_runs_that_wait`

**C-changes-03** The office tools reach only the files of the calling worker's own team's folder and of the teams within it, the top team's worker's every file of the office: a path outside that reach is refused by `read_file`, `write_file` and `remove_file`, saying why, is not listed by `list_files` nor searched by `search_files`, and is never put: a `put_draft` whose draft holds such a file, as one drafted before the worker's team moved, is refused, saying which. The worker's team, and what it reaches, are found in the office as it was last read well, the one the worker runs in, not in the folder as it is at the call. A team's worker must not read or write the files of the team around it, nor of a team beside it. *Test:* `changes_office_tools_reach_own_team_and_within`

## The draft

**R-changes-09** A draft belongs to the request the worker is working on, and outlasts the run that made it.
*Accept:* the builder's run writes a file into the draft and ends; a later run on the same request lists that file in `list_files` with the same text. A server stopped and started between the two runs makes no difference.

**R-changes-10** For each file it touches, a draft holds the whole new text or that the file goes, and the text the file had in the folder when it was first drafted. A file written into the draft again after it changed in the folder is drafted anew, against its text then, and the worker is told so.
*Accept:* the builder writes a file twice; the draft holds the second text, and as the text it was drafted against, the folder's text from before the first write.

**C-changes-04** Writing a file's present text into the draft, or removing a file that only the draft made, takes that file out of the draft. The draft, and the files a decider is shown, hold only what changes. *Test:* `changes_unchanged_file_leaves_the_draft`

**C-changes-05** A draft holds only files of the office: a path outside the folder, a path with a part that begins with a dot, Bulldog's store, the repository and the files Bulldog ships are refused, saying why. What Bulldog ships and keeps is not the office's to change, and a file that is written but never read would mislead whoever decides. *Test:* `changes_draft_refuses_what_is_bulldogs_own`

**C-changes-06** A draft is seen and changed only from its own request. Two workers drafting on two requests must not write over each other's work, and a draft holds what the request's team read. *Test:* `changes_drafts_are_kept_apart`

## A note

**R-changes-34** A note's write that Work hands here, with its team, its name, its whole new text, its why, the worker that wrote it and the request (R-work-25), is written as `notes/<name>.md` in that team's folder and committed, by that worker, with its why and the request, and the commit is given back. As the notes tools ship, no decision is asked; held by a copy that marks it, its job runs only after its yes (C-changes-51).
*Accept:* the steward, whose definition names the notes tools, writes the house's note `fuses` with a why. `notes/fuses.md` holds the text, the history's newest commit is by `steward` and holds that one file, nobody was asked to decide, and the commit's id is given back to Work.

**R-changes-37** A note's commit has the first line of why as its first line, the rest of why after it, then "What it does:" with one line naming the team and the note, then `Request`; written at once, it has no `Decided-by` and no `Note`.
*Accept:* the steward's write of `fuses`, with a why of two lines in request 14, is a commit whose subject is the first line, whose body holds the second, "- the house's note fuses is new." and `Request: 14`, read back from the repository alone; the screens' history shows it by the steward, with its why, decided by nobody.

**C-changes-49** A note's write stands apart from the request's draft: it is not added to it, and it neither puts nor withdraws a change put from the draft. A change drafted against the note's old text and written later finds the file changed, and is not written (C-changes-26). A worker writing a note must not put another's unfinished draft before a person, nor take away what waits. *Test:* `changes_a_note_stands_apart_from_the_draft`

**C-changes-50** A note's write is written and committed by the same steps as a change, with only the decision left out: whole beside itself and moved into place, never over an edit by hand not yet committed, in a commit that holds the one note and no other file, by name with no identity of a real person. A second way of writing the office would drift from the first. *Test:* `changes_a_note_goes_the_one_way`

## Checking a change

**R-changes-11** A change is checked by reading the office as it would make it, by the office's own reading, with every worker, rule and bridge machine checked by Corbie, and every problem said together.
*Accept:* a change that names an unknown person in a team file, gives two workers one name, writes a worker Corbie refuses, and writes a worker whose `model: quick` no team's `models` gives is given all four problems at once, each with the file it is in.

**C-changes-07** A change is refused for a problem the office as it is does not have. A problem the office already has is said beside, as standing, and does not refuse a change that leaves it as it was; where the office as it is cannot be read at all, only a change after which it reads is accepted. One broken worker must not stop every change, and no change may make the office worse. *Test:* `changes_a_standing_problem_does_not_refuse`

**C-changes-08** Each problem is said once, even where two parts of the reading find it. The same complaint twice reads as two things wrong. *Test:* `changes_a_problem_is_said_once`

**C-changes-09** A worker's draft that adds or takes away a person of the office, or gives, changes or takes away an identity by which a person is reached, is refused; a person's is not, and is decided as any change is, a change to their own entry included. A person's yes is accepted from their identity, so a worker that could write one could say yes as a person; and changing one's own identity on a bridge is what someone taking over an account would do. *Test:* `changes_a_worker_cannot_change_the_people`

**C-changes-53** A worker's draft that gives, changes or takes away a person's account by a sign-in method or their `password`, or adds, changes or takes away a sign-in method's entry under `signin`, or writes any file under `signin/`, is refused; a person's is not, and is decided as any change is. Whoever holds an account a person's entry writes signs in as them, and a method's machine says who signed in, so a worker that could write either could sign in as anyone. *Test:* `changes_a_worker_cannot_change_how_anyone_signs_in`

**C-changes-10** A worker's draft that writes, changes or takes away the `put_draft` list, the `everything` or, while its `everything` is taken from them, the `people` of the team whose decision the draft is (R-changes-17) or of any team around it, or any file of a rule or worker on those lists, or the `models` entry from which such a worker's model named by one word is filled in, is refused; a person's is not. A change must never be judged by deciders it wrote itself. *Test:* `changes_a_worker_cannot_change_who_decides_a_change`

**C-changes-11** A worker's draft that adds, changes or removes a bridge's in machine, or any file it uses, is refused; so is one that adds or removes a bridge's entry in `office.yaml`, or changes its `from` or any of its settings but its places; a person's is not. A worker's change to a bridge's places is drafted as any change, and set apart (C-changes-14). An in machine says who a person is, so a rewritten one could give a yes in any person's name; and a bridge's entry says what it runs, where it listens and whom it signs in as, so a worker could send its token to another host or sign it in as someone else. *Test:* `changes_a_worker_cannot_set_a_bridge`

## What a change says

**R-changes-12** A change says, of teams: each team new, gone, or moved inside another; each change to a team's `about`, its people, its lead, its heed and its settings, its decider lists, its disclosure check, its routines, its `models` and its notes; and each change to the office's `zone` and `runs`.
*Accept:* a change that makes a team, moves another inside a third, rewrites a team's `about`, names a person in a team, changes a lead, gives a team a heed, adds a decider, takes away a disclosure check, adds a routine, gives a team's `models` a new name, rewrites a notes file, and sets the office's `zone` and its `runs` is given one sentence for each, naming the team.

**R-changes-13** A change says, of people: each person new to or gone from the office, and each identity given, changed or taken away, naming the bridge.
*Accept:* a person's change that adds a person with a Slack identity and changes another's Telegram identity is given a sentence for each, naming the person, the bridge and the identity.

**R-changes-14** A change says, of workers: each worker new, gone or moved; and each change to its model, as Bulldog fills it in where it is named by one word, with the model's service, address and the secret it names; to what it may reach; to the secrets it holds and where each may be sent; to its tools, a flow's stages or a statechart's jobs (R-office-39), each said as such; and to which of them wait for a yes. A change to a team's `models` is said for each worker whose model, as filled in, it changes.
*Accept:* a change that adds a worker and, in another, changes its model's address, adds a host it may reach, binds a secret to that host, adds a tool and stops a tool waiting for a yes is given a sentence for each, naming the worker. A change that renames the bins flow's stage `which` to `sort`, marked for approval, says the bins has the stage sort, which waits for a yes, and no longer has the stage which; a statechart `porch.yaml` new to the house is said to have the job `asking` and the job `lock`. A change that gives the house's model `usual` a new address is given a sentence for each worker of the house written with `model: usual`, and none for a garden worker written with `model: usual` where the garden's `models` gives `usual` itself.

**R-changes-15** A change says, of rules and bridges: each rule new, gone, or with its text or a file it uses changed, with the lists, disclosure checks and heeds it is on, of each team that names it, and, for a rule that is a definition, each change to what it may reach and the secrets it holds; and each bridge new or gone, and each change to its in or its out machine, to what either may reach, and to its settings, with each place it listens in that is new, gone or changed, the team asked there and whether guests may ask there.
*Accept:* a change that rewrites a bare script on a team's decider list, lets a rule that is a definition reach a new host, changes a bridge's out machine to reach a new host and adds to the Slack bridge's settings a place that names platform and lets guests ask is given a sentence for each, the first two naming the team and the list, the last naming the bridge, the place, platform and that guests may ask there.

**R-changes-43** What a change does to how people sign in is said: an account given, `sam signs in as sam@example.com by google`, changed, with what it was, or taken away; a person who may now set a password, or no longer may, whose password is then destroyed; a sign-in method new, gone, its machine changed, what it reaches, and its settings changed. Each is set apart as letting more by but those that take away (C-changes-14).
*Accept:* the house signing in by `google` with Sam's account: giving Kit `google: kit@example.com, password: true` lets by "kit signs in as kit@example.com by google" and "kit may set a password to sign in with"; changing Sam's lets by his new account with the old; taking his away says he no longer signs in by google and may no longer sign in with a password, letting nothing more by; adding `entra` lets by that it is new and that its machine may connect to `login.microsoftonline.com`; changing google's `client_id` lets by that its settings are changed; taking google away says it is gone.

**R-changes-16** The sentences that let more by are set apart from the rest, wherever a change is shown: to the worker by `check_draft`, to whoever decides, to a person drafting on the screens, and in the commit.
*Accept:* a change with one sentence that lets more by and one that does not shows them apart in all four places, the same sentences in each.

**C-changes-12** What a change says is read from the files as they would be, and from nothing whoever drafted it said. Whoever drafted it could describe the change as smaller than it is. *Test:* `changes_said_from_the_files_alone`

**C-changes-13** Every file in a change is in at least one sentence; a file Bulldog has nothing particular to say of is said to be new, changed or gone, by its path. Nothing may change unsaid. *Test:* `changes_every_file_is_said`

**C-changes-14** A sentence lets more by exactly when: a definition may reach something it could not; a secret is held that was not, or may go somewhere it could not; a worker's model, written out or filled in from a name, is at a new address or names a new secret; a tool, a flow's stage or a statechart's job no longer waits for a yes; someone sees what a team reads who could not, because a person is named in it, comes to stand as the people of a team with none, or a worker moves into it; an identity is given or changed; an account by a sign-in method is given or changed, or a person may now set a password; a sign-in method is new, or its machine, what it reaches or its settings change; a decider is taken off a list, a rule or a worker is put before the first person on a list, a rule or a worker on a list no longer advises, or a list is taken away; a disclosure check is taken away; a rule on any list or disclosure check has its text, or a file it uses, changed; or a bridge is new, or its in or out machine, its `from` or any of its settings, its places among them, change. Setting apart too little lets a widening by unseen, and setting apart too much teaches a person to ignore it. *Test:* `changes_what_lets_more_by`

**C-changes-15** What narrows, such as a decider added after a person, a permission taken away or a tool, stage or job that now waits for a yes, is said and never set apart. A narrowing set apart reads as a risk that is not there. *Test:* `changes_a_narrowing_is_not_set_apart`

**C-changes-16** What a definition may reach, which secrets it holds and which of its tools, stages or jobs wait for a yes is taken from Corbie's reading of the definition, with every file it names, and never from a reading of Bulldog's own. Two readings of one definition would disagree. *Test:* `changes_reach_read_through_corbie`

**C-changes-17** A worker whose definition goes from one team's folder while one of the same name comes into another's is said once, as a move, and not as one leaving and one joining. A move said as two things hides what it is. *Test:* `changes_a_move_is_said_once`

**C-changes-18** A worker that moves, whoever drafted the move, takes, in the same change, the files only its definition uses; they are added to the draft and shown with it. A file its moved definition still names where it is stays there. What a worker uses goes with it, or is left behind for nothing to read. *Test:* `changes_a_moved_worker_takes_what_is_its_alone`

**C-changes-19** A file a moving worker takes is never written over one already in the new team's folder; the change is refused, naming the file. Writing over it would lose another's file. *Test:* `changes_a_move_never_writes_over_a_file`

## Whose decision a write is

**R-changes-17** A held `put_draft` or `write_doc` is a decision of the innermost team whose folder holds every file it writes; but where it touches a team file, a `team.yaml`, it is a decision of the team around that team, and where it touches `office.yaml`, of the top team. It is asked of that team's `decide` list named for the tool, or else its `everything`, and walked on outward as any held action is (R-decisions-07).
*Accept:* a garden worker's held `put_draft` of a new `garden/weeder.yaml` is the garden's, asked of the garden's `everything`; its put of `garden/team.yaml` alone, or of both, is the house's; the house's builder's put of `garden/weeder.yaml` is the garden's, and of `office.yaml` and `garden/weeder.yaml`, the house's. With `put_draft: [./no-secrets.js, sam]` in `office.yaml`, a put of `office.yaml` is first given to the rule.

**C-changes-52** A write that touches a team file is never decided by that team: a `team.yaml` is decided by the team around it, and `office.yaml`, which has none around it, by the top team. A team that decided a change to its own team file could rewrite its own deciders, and so decide what it may do. *Test:* `changes_a_team_file_is_decided_around_it`

**R-changes-36** A held write's list is walked as any held action's: a rule's or a worker's yes approves it, a refusal ends it, a referral goes on with its note, and one that advises refuses or recommends yes (R-decisions-41); a team that wants a person to decide its writes puts one on the list, and one that wants a check first marks it to advise, as `put_draft: [reviewer: advise, sam]`. A held write whose job runs with no yes kept for it is not written, and its result says so.
*Accept:* with `put_draft: [reviewer: advise, sam]` in `office.yaml`, a put of `office.yaml` asks the reviewer, whose yes reaches Sam as "reviewer recommends yes", and is written only on Sam's yes; with `put_draft: [./no-secrets.js]` and a rule that says yes, it is written with nobody else asked, decided by the rule.

**R-changes-38** The people who decide a change are the people the top team's `put_draft` list names; where it names no person, those its `everything` names; where neither names a person, the top team's people. They alone put a change from the screens (C-changes-38) and read the history (C-changes-45).
*Accept:* with `decide: { everything: [kit], put_draft: [./no-secrets.js, sam] }` in `office.yaml`, Sam may put a change and read the history, and Kit may not; with `put_draft: [./no-secrets.js]` and `everything: [kit]`, Kit may and Sam may not; with no `decide`, Sam and Kit both may.

**R-changes-18** Whoever decides a held write is shown why, in the words of whoever drafted it; what the change says, with what lets more by set apart; every file whole beside what it replaces; and the request it was made in, on the screens and in the direct message a bridge carries, where each file is given by the lines it changes.
*Accept:* a person asked to decide a change that changes one file and adds one is shown the why, the sentences in two groups, the old and new text of the first and the whole of the second, and the request; the direct message on their bridge holds the why, the sentences and each file's lines taken away and added.

**R-changes-19** A refused write writes nothing; Corbie tells the worker its job was denied, with who refused and their note, and the draft is kept. A write whose walk comes to nobody is refused by Bulldog, with why as its note (R-decisions-38).
*Accept:* after Sam refuses a held put, the folder and the history are as they were, the builder's model is told it was denied, with Sam's note, and `list_files` in the same request still lists the drafted files.

**C-changes-21** A write is decided by the lists, rules and workers of the office as it is, never as the write would make it. A change that rewrites the rule judging it would otherwise judge itself. *Test:* `changes_judged_by_the_office_as_it_is`

**C-changes-22** A held write's files are fixed when Corbie holds its job: the draft as it is then, checked, or the one doc it writes, and the call that runs after its yes writes exactly those. A decider says yes to what they were shown, and to nothing else. *Test:* `changes_a_yes_writes_what_was_shown`

## Writing it

**R-changes-21** On a yes, the change is read once more, written, committed, and its draft cleared; the office is then read again, and everything begun after is begun in the office as it now is.
*Accept:* a change that makes a worker the house's lead is approved; the draft is empty, the history's newest commit holds it, and the next thing said in the chat goes to that worker.

**R-changes-22** A worker that a written change takes away has its runs cancelled.
*Accept:* a worker is mid-run when a change that removes it is written; its run is cancelled, and no new run of it starts.

**C-changes-25** A change is read with the office again when it is to be written, and is not written if it would now leave a problem the office does not have. The folder may have moved on since the change was put. *Test:* `changes_checked_again_when_written`

**C-changes-26** A file whose text in the folder differs from the text it was drafted against is not written over, and nothing of the change is written; the outcome names each such file. A hand edit or a later change must not be lost. *Test:* `changes_a_stale_file_is_refused`

**C-changes-27** A change is not written while any file it touches differs from the last commit by an edit by hand, even where the edit came after the change was put; the outcome names the files and says to commit or undo the edit first. Otherwise the hand edit would be committed under the name of whoever drafted the change. *Test:* `changes_a_hand_edit_is_never_folded_in`

**C-changes-28** Every file of a change, removals included, is written or none is: if any fails, every file already written is put back as it was. A change half written is an office nobody decided. *Test:* `changes_all_files_or_none`

**C-changes-29** Each file is written whole beside itself and moved into place. A reader, the screens or a run, must never see half a file. *Test:* `changes_a_file_is_never_half_written`

**C-changes-30** A commit that cannot be made puts the change's files back as they were. Every change to the files is a commit, so nothing may stay written that is not kept. *Test:* `changes_a_failed_commit_puts_the_files_back`

**C-changes-31** Anything that stops a write after its yes ends it as not written, with why, given to whoever drafted it as the tool's result, and the draft is kept. A yes that silently writes nothing leaves whoever drafted it never told. *Test:* `changes_an_unwritten_yes_ends_the_decision`

## The commit

**R-changes-23** A commit's first line is the first line of why; the rest of why follows; then "What it does:" and a line for each sentence of the change that does not let more by, and "What it lets more by:" and a line for each that does (R-changes-16); then `Decided-by` and `Note`, where its job was held or a person put it, and `Request`.
*Accept:* a change put with a why of two paragraphs and decided by Sam with a note is a commit whose subject is the first line, whose body holds the rest, the change's sentences under "What it does:" and "What it lets more by:", and the three notes, each read back from the repository alone.

**R-changes-24** A commit is by the worker or person who made the change, by name, and a person's own change names them as who decided.
*Accept:* the builder's approved change is by `builder`, decided by `sam`; Sam's own change from the screens is by `sam`, decided by `sam`.

**C-changes-32** The first line of why is not repeated as the first line of the body. A commit that says its subject twice reads as a slip. *Test:* `changes_a_commit_says_its_subject_once`

**C-changes-33** A commit's notes are its last lines, each a key with no space in it, a colon and a value; a note with no value is not written, a newline in a value becomes a space, and each line of what it does begins with "- ", so it is never read as a note. Who decided, their note and the request are read back from the history alone, by anything that reads git. *Test:* `changes_commit_notes_read_back`

**C-changes-34** A commit holds the change's files and no others: it is made from the last commit and exactly those files, whatever else is staged in the repository. What a person staged by hand, in the office or beside it, is theirs, and must not be committed under another's name. *Test:* `changes_a_commit_holds_only_its_files`

**C-changes-35** A commit names its author and committer by the worker's or person's name, with an address that is the same fixed placeholder for everyone; no identity by which a person is reached, and no address of a real person, goes into the history. The history is read with anything and goes wherever its people take it. *Test:* `changes_the_history_holds_no_identity`

**C-changes-36** The repository is read and written with a library, and no git program is run. A program may not be there, and running one is not Bulldog's to do. *Test:* `changes_history_kept_with_no_git_program`

**C-changes-37** Bulldog never pushes, pulls, or names a remote. Where the repository goes is its people's. *Test:* none: Bulldog holds no way to reach a remote, so there is nothing to call.

## A person's change

**R-changes-25** A person drafts a change on the screens by writing or removing whole files, setting one field of a file, moving a worker to another team, or taking a worker away, or from a draft of those another screen started, as Use and Copy of a thing Bulldog ships do (R-server-61), which is theirs to change as any; and is shown, as they go, every problem, what the change says with what lets more by set apart, and every file as it would be.
*Accept:* a person sets one field of a worker's definition and is shown that file as it would be, the sentence the change makes, and no problems; they then set a field Corbie would refuse and are shown the problem, with where it is.

**R-changes-26** A person's draft is held by their screens, and Bulldog keeps nothing of it until it is put.
*Accept:* a person drafts a change and closes the screen without putting it; the folder, the history and the store are as they were.

**R-changes-27** Moving a worker takes its definition and the files only its definition uses to the new team's folder, and points what its definition names to files that stay where they are.
*Accept:* a worker whose definition names a script only it uses and a script another worker uses is moved; the change holds its definition and its own script in the new folder, the shared script stays, and the moved definition names the shared script where it is.

**R-changes-28** Taking a worker away removes its definition and the files only its definition uses; what another definition uses stays.
*Accept:* a worker whose definition alone uses one script, and shares another, is taken away; the change removes the definition and the first script, and leaves the second.

**C-changes-48** A worker that is taken away takes with it, in the same change, the files only its definition uses, such as the script of a flow that runs one script; a file another definition uses is never taken. A worker removed without them leaves files behind that nothing reads, and one removed with a shared file breaks another. *Test:* `changes_a_worker_goes_with_what_it_alone_uses`

**R-changes-29** A person who decides a change puts their own, and it is decided as if they had called `put_draft`: by the team whose decision it is (R-changes-17), from its `put_draft` list or else its `everything`. Every rule and worker that walk asks before its first people is asked, and may refuse it; when they have referred it or recommended yes, it is written with the yes of the person who put it where they are among those people, and is asked of those people where they are not. Where the walk asks nobody before people the person is among, it is written as it is put.
*Accept:* in the house, whose `everything: [sam]` asks nobody before Sam, Sam puts a change and it is written and committed by Sam, decided by Sam, with nobody asked. With `put_draft: [./no-secrets.js, sam]`, Sam puts a change of `office.yaml`; the rule is run and refers it, and the change is written and committed by Sam, decided by Sam. With a rule that refuses, nothing is written and Sam is shown the rule's note. With the garden's `everything: [kit]`, Sam's change of `garden/weeder.yaml` is asked of Kit, and written on Kit's yes.

**C-changes-38** Only the people who decide a change put one from the screens; anyone else asks for it in a request, a worker drafts it, and it is decided as any change is. Every change is decided by someone who may decide it. *Test:* `changes_a_person_who_does_not_decide_cannot_put_one`

**R-changes-35** A bridge Bulldog ships is set by a person's change that writes its entry in the `bridges` of `office.yaml`, with the `from` that names its folder and the settings it is shipped with, as its page and its `settings.yaml` give them (R-bridges-36), the field Use sets (R-server-61), under the bridge's name or, when that is taken, the first after it that is free, as `slack-2`; nothing of its folder is copied into the office; it is checked, decided and written as any change, and runs as the Bulldog that serves the office ships it.
*Accept:* Sam sets the field `bridges.telegram` of `office.yaml` on the screens to `{ from: ./.bulldog/bridges/telegram, places: { direct: {} } }`, the settings Bulldog ships it with (R-bridges-36), which hear people's direct messages to the bot; the draft holds `office.yaml` alone, with `telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }` under `bridges`; the change says the bridge is new and sets it apart, and once written, the commit holds `office.yaml` alone and the bridge runs `.bulldog/bridges/telegram`. A worker that drafts the same entry is refused, by C-changes-11.

**R-changes-39** A person copies a thing Bulldog ships into the office by a change of their own, which Copy drafts (R-server-61): a bridge's machines, its script and its page are written into `bridges/<name>/`, and its entry set in `office.yaml` with its settings as written and its `from` taken out, in the one change, for each entry that runs it, or for a new entry with the settings it is shipped with when none does; a rule's files but its page are written into the folder of each team that names it, and each line of the team's file that names it, its heed, its disclosure or a list of deciders, names the copy. The copy is then the office's own, and a later Bulldog does not change it (R-bridges-36). A tools file is not copied: a tool is narrowed by writing it over the call Bulldog ships (R-work-20), and Bulldog answers only the tools it ships (C-work-23).
*Accept:* in the work office with `heed: ./.bulldog/rules/for-us/for-us.yaml` in `office.yaml`, Al's draft of Copy of for-us writes `for-us.yaml` and `for-us.js` and sets the heed to `./for-us.yaml`, and its check can be put; his draft of Copy of the Telegram bridge, which no entry runs, writes `bridges/telegram/` without `settings.yaml` and sets `telegram` with `places: { direct: {} }` and no `from`, and its check can be put. Copy is offered for no tools file.

**C-changes-39** A person's change is checked, written and committed by the same steps as a worker's; nothing a person puts skips the check, and it may hold any file a draft may. Two ways of writing the office would drift apart. *Test:* `changes_a_persons_change_goes_the_one_way`

**C-changes-40** What a person puts is written exactly as they were shown it; if the folder changed since, nothing is written, and they are told to look again. A person says yes to what they saw. *Test:* `changes_a_person_writes_what_they_were_shown`

## Rewriting part of a file

**R-changes-30** Where Bulldog changes part of a file a person wrote, only the lines that change are changed.
*Accept:* setting one field in a definition of forty lines changes one line of its text, and every other line is as it was.

**C-changes-41** A rewritten file keeps its comments, its blank lines, how far it is set in, its line endings, how it ends, and the spacing inside each line it already had; a new line written into a mapping set out between braces is spaced as the file spaces them. Only what changes should change, as a person wrote the rest. *Test:* `changes_a_rewrite_keeps_how_a_file_is_set_out`

**C-changes-42** A blank line within a block of text keeps its place and the block keeps its value. A blank line there is part of the value, not of the layout. *Test:* `changes_a_rewrite_keeps_a_block_of_text`

**C-changes-43** A rewrite is used only when it reads as the same value as the file written plainly; otherwise the plain text is used, and the person is told the layout was not kept. Keeping a layout must never change what a file says. *Test:* `changes_a_rewrite_is_checked_against_the_plain_text`

**C-changes-44** A file that marks where a document begins or ends, with `---`, `...` or a line that begins `--- `, is never rewritten in part; the person is told to change it whole. A rewrite would lose the marks or the documents after them. *Test:* `changes_a_file_with_document_marks_is_not_rewritten`

## The history

**R-changes-31** The screens show the history: each change with who made it, why, who decided and their note, and the request; each note's write with who wrote it, why, and the request; and each file as it was and as it is, a file that is not text being said to be so.
*Accept:* after the builder's approved change, a change of Sam's and the steward's write of a note, the history lists all three, newest first, each with its who and why, the first two with who decided and the note's as decided by nobody; opening the first shows each of its files before and after.

**R-changes-32** An office in a folder within a larger repository has the history of its own files: its changes, its commits and its edits by hand are those of its folder.
*Accept:* an office in a folder of a larger repository, beside a file outside it that has its own commits and an edit by hand, lists only the office's changes, and only the office's files as edited by hand.

**C-changes-45** The history, and each change and note in it, is seen on the screens only by the people who decide a change. It holds every file of the office as it was, every team's notes among them, and may hold what no team's people alone should read. *Test:* `changes_history_seen_only_by_who_decides_a_change`

## Edits by hand

**R-changes-33** The screens say which files of the office are not as the last change left them, until they are committed by hand.
*Accept:* a person edits a team file by hand; the screens list it as edited by hand; once it is committed outside Bulldog, it is no longer listed.

**C-changes-46** Bulldog never commits an edit by hand, by a change or by a note's write, except the office's first commit, when the server makes a repository for an office that has none and commits what is there. What a person changes by hand is theirs to commit. *Test:* `changes_a_hand_edit_is_not_committed`

**C-changes-47** Which files are edited by hand is read from the folder when it is asked, and is right at once after a change is written. A list that lags shows a file Bulldog just committed as changed by hand. *Test:* `changes_hand_edits_are_current_after_a_change`

## Interfaces

- **Office** reads the office with a change laid over it, as it reads it always, and gives every problem once, each worker's model as filled in from the nearest team's `models`, what each worker runs by name with whether it waits for a yes, and each place a bridge's settings list with the team it names; it says which files are the office's and each team's folder and team file, and reads the top team's `put_draft` list, its `everything` and its people, of which `everything` always names a person.
- **Decisions** takes a held `put_draft` or `write_doc` as any held job, has it fixed here first, with the team whose decision it is (R-changes-17), refuses as Bulldog one that cannot be put (C-changes-24), walks that team's list for the tool and asks its people as for any decision, accepts a person's yes only from their checked identity, and tells this area how it was decided, so the write after its yes names who decided (R-changes-36).
- **Work** answers the tools Bulldog ships as runs that wait on tickets, every one of them a tool Bulldog does (C-work-23); gives `write_note` to a worker whose definition names the notes tools, and each of its runs the team's notes, and says which note a run may write, and hands each write here to be committed (R-work-25); hands `write_doc` and the office tools here with the worker's team; and cancels the runs of a worker a written change takes away.
- **Requests** keeps each draft with its request, and the change and its decision in what was done.
- **Bridges** reach the people who decide a change; a bridge Bulldog ships is set by its entry, with its `from`, as a change (R-bridges-36), with the settings it is shipped with, the Telegram bridge's hearing people's direct messages to the bot; a bridge's entry, its `from` and its settings but its places, and its in machine, are a person's to write (C-changes-11); and the shape of a bridge's settings and its places is Bridges'.
- **Server** gives the screens a person's draft, its check and its putting, and only to the people who decide a change (R-changes-38); shows them the history (C-changes-45) and the edits by hand (R-server-35); writes the tools files Bulldog ships at every start as the program has it (R-server-22); and makes the repository of a folder that has none, with the office's first commit (R-server-07), or one of the office's own for an office it has only now written inside another repository; and is told what this area does not allow as a refusal, a change the history does not hold as not there, and anything else as a failure, which it answers as such (C-server-36).
