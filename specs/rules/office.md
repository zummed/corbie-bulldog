# Office: rules

The detailed rules behind the [Office overview](../office.md). Requirements are `R-office-nn`, constraints `C-office-nn`. "The house" and "the work office" are the two offices the overview shows as files; the house is the fuller example there, with its Telegram bridge, and not the office the server writes into an empty folder, which is the house without the bridge (R-server-06). A test writes them, or a copy changed as the rule says, into a folder of its own and reads it.

## The folder

**R-office-01** The office is the folder Bulldog is given, and its top team's file is `office.yaml` at the top of it; a folder that is not there, or that holds no `office.yaml`, is refused with that said, and nothing is written into it.
*Accept:* reading a path that does not exist reports that it is not there. Reading a folder that holds files but no `office.yaml` reports that it holds no `office.yaml`, and the folder holds the same files, unchanged, afterwards.

**R-office-02** A team within is a folder directly inside a team's folder that holds a `team.yaml`, and it may hold teams of its own the same way.
*Accept:* the house with `garden/beds/team.yaml` added reads three teams: `office`, `garden` within it, and `beds` within `garden`.

**R-office-03** A worker is each file directly in a team's folder whose definition is an agent, a flow or a statechart; it is a worker of that team and is named by its file's name without the extension.
*Accept:* the house reads `steward`, `fixer`, `sparks`, `clerk`, `bins` and `builder` as the top team's workers and `gardener` and `groundsman` as the garden's. `crew.yaml`, a tools file, `small-orders.js` and a flow at `tools/check.yaml` are not workers. In the work office, `payments/no-account-numbers.yaml`, a flow named as a rule, is not a worker (C-office-22).

**R-office-04** Nothing whose name starts with a dot is the office: such a file or folder is no team, worker, note or bridge, and is not among the office's files.
*Accept:* the house with `.git/`, `.bulldog/` and `.scratch/team.yaml` reads the same teams and workers as without them, and none of their files is among the office's files.

**C-office-01** A `team.yaml` anywhere but directly in a team's folder, an `office.yaml` anywhere but at the top, and a `team.yaml` at the top, are each a problem of the reading. A team file that is not read is a team that silently does not exist. *Test:* `office_stray_team_file_refused`

**C-office-02** Everything a team file names is inside the office folder, and Corbie is given the office's files and what Bulldog ships at `.bulldog/`, as the program holds it, and nothing else, to read a definition from. A worker or a rule read from outside the folder would run while being in no history and no re-reading. *Test:* `office_nothing_named_outside_folder`

**C-office-03** What Bulldog ships is found at `.bulldog/` at the top of the office by its path there, a file in a shipped thing's folder as one at the top, such as `.bulldog/tools/talk/talk.yaml`, and by nothing else: no folder of the office is ever taken for Bulldog's because of a word in its path. Found by a word, an office file whose path held it would be read from Bulldog's files instead, with no problem said. *Test:* `office_own_files_by_path_only`

## A team

**R-office-05** A team file takes `name`, `about`, `people`, `lead`, `decide`, `disclosure`, `routines` and `models`; the top team's file also takes `zone`, `everyone`, `bridges` and `runs`, which is how many runs may be at work at once in the whole office and is a whole number from 1, and `change` under `decide`.
*Accept:* the house and the work office read with no problem, `models` in `office.yaml` and in `garden/team.yaml` among them, and so does the house with `runs: 2` in `office.yaml`; `runs: 0` there is a problem, and so is `budget: 40`, naming `budget`. Each of `zone`, `everyone`, `bridges`, `runs: 2` and `decide: { change: [sam] }` in `garden/team.yaml` is reported as a problem that names the key and the file.

**C-office-04** A key a team file does not take, or one it takes only at the top written in a team within, is a problem of the reading, never read past. A key read past is a setting a person believes is in force and is not. *Test:* `office_unknown_key_refused`

**R-office-06** A team is known by its folder's name and the top team as `office`; a team's `name` is what it is called, and a team that has none is called by its folder's name.
*Accept:* the work office's teams are known as `office`, `platform`, `payments` and `support`, and called "Platform support", "Platform", "Payments" and "Support". With `name:` removed from `support/team.yaml`, it is called "support".

**C-office-05** Nothing is found by a team's `name`: it may be in any words, and changing it changes no team's identity. A key made from a name in any words can come out empty, and one made from a name that changes loses sight of what was found by the old one. *Test:* `office_name_is_only_words`

**R-office-07** A team's people are exactly those its `people` names, each a person under `everyone`; being in a team around it puts nobody in it.
*Accept:* in the work office, the people of `payments` are `lee` and `priya`, those of `office` are `al`, and `al` is in no other team. `people: [sam, nobody]` in the house is a problem naming `nobody`.

**R-office-08** A team's `lead` names one worker or one person of that team, and every team has one.
*Accept:* in the house, `lead: groundsman` in `office.yaml`, a `team.yaml` with no `lead`, and `lead: [gardener, groundsman]` are each a problem. In the work office, `support`'s lead is `dana`, a person.

**R-office-09** A team need write no `decide`: a team that writes no `everything` list has its own people as that list.
*Accept:* `garden/team.yaml` without `decide` reads, and its `everything` is Sam. `support/team.yaml`, which has no workers and no `decide`, reads. `platform/team.yaml`, which has no people and no `decide`, reads, and its `everything` is empty. `office.yaml` without `decide` reads, and its `everything` is Sam and Kit; with `decide: { everything: [./small-orders.js] }` it is a problem of `office.yaml`, by C-office-06.

**R-office-10** Under `decide`, each key is `everything`, the name of a tool, or at the top `change`, and each value is an ordered list of deciders; `disclosure` is one such list.
*Accept:* the house's `decide` reads as two lists, `everything` and `order_part`, in the order written, and the work office's `disclosure` for payments reads as one list holding the rule `./no-account-numbers.yaml`. `decide: { everything: sam }` and `disclosure: { lee: yes }` are each a problem.

**R-office-11** A decider is a person under `everyone`, a worker of any team, or a rule, which is a path from the team's file to a bare script or a Corbie definition in the team's own folder, and not in a team within it, or to a rule Bulldog ships (R-office-35); a list may name no person, only rules and workers; a name that is none of these, a path to nothing, and a decider written twice in one list are each a problem.
*Accept:* `disclosure: [./no-account-numbers.yaml]`, a definition, reads in `payments/team.yaml`, and so does the house's `order_part: [./small-orders.js, sam]`, a bare script before a person; with `sam` removed, `[./small-orders.js]`, a list with no person in it, reads too. `[./missing.js, lee]`, `[nobody]`, `[lee, lee]`, and `[../bot.yaml]`, a path out of the team's folder, are each a problem naming the entry. In the house, `order_part: [./garden/check.js]` in `office.yaml`, naming a script in the garden's folder, is a problem. `decide: { everything: [bot, al] }` in `platform/team.yaml` reads, `bot` being a worker of the top team.

**C-office-06** The top team's `everything`, as written or as taken from its people, names at least one person; one that names none is a problem of `office.yaml`. This is the one check of a reading that a person is reached: a decision that every decider passes, and a team with no people, go out to the office's `everything` in the end, and a person must be there for it to reach. *Test:* `office_top_everything_names_a_person`

**C-office-20** In a list, every rule and worker comes before every person; a list with a rule or a worker after a person is a problem naming the team and the list. A decider after the people could never be reached. *Test:* `office_people_last_in_list`

**C-office-21** A list names only people of the office, workers of the office, rules in the team's own folder and rules Bulldog ships; any other entry is a problem naming the team, the list and the entry. A misspelt decider would otherwise be passed over at the moment it matters. *Test:* `office_unknown_decider_is_a_problem`

**C-office-22** A definition that a `decide` or `disclosure` list names by its path is a rule and never a worker, wherever in the team's folder it sits. Read as a worker too, it could be taken for a colleague or a lead and started with the allow lines written for judging. *Test:* `office_rule_definition_is_not_a_worker`

**C-office-23** A rule may do only what its own file says: a bare script is given its decision and no permission, and a definition only what its own allow lines allow; Bulldog grants a rule nothing and writes no definition around one. A bare script is wrapped by Corbie's client, in a flow of one stage that runs it with no permission. What a rule may reach is then read where it is written, and judged when it changes. *Test:* `office_rule_granted_nothing`

**R-office-35** A rule may be one Bulldog ships, in a folder of its own under `.bulldog/rules/` at the top of the office, and any team may name it in a list by its path from the team's file, as `./.bulldog/rules/<name>/<file>` in `office.yaml` and `../.bulldog/rules/<name>/<file>` in a team directly within. It is read from what Bulldog ships, never from the office's files, and sent as it ships: it names no model by one word and holds its own allow lines, so it is the same rule whichever team names it, and is checked once; a problem Corbie finds in it is reported on each team that names it. It is of no team: like any rule, it decides as the team whose decision it is, its run labelled with that team (R-work-47) and its word in a request's thread that team's reading, seen by that team's people and not by the people of another team that names it. A path into `.bulldog/` that is not a rule Bulldog ships in `rules/`, such as its talk tools, is a problem naming the entry.
*Accept:* `order_part: [./.bulldog/rules/none/none.js, sam]` in the house's `office.yaml` is a problem saying Bulldog ships no such rule, and so is `[./.bulldog/tools/talk/talk.yaml, sam]`. In `garden/team.yaml`, `everything: [../.bulldog/rules/none/none.js, sam]` is the same problem, and not one of a path leaving the garden's folder, while `[./.bulldog/rules/none/none.js, sam]` there names `garden/.bulldog/`, which is no file of the office (C-office-03).

**R-office-12** A `decide` key that names no tool held by any of its team's workers is reported on its team and used for nothing, and the rest of the office reads; while one of the team's workers cannot be read, no key is reported for naming no tool. A key that begins with a `list:` entry's name and `_`, such as `github_create_issue`, names a tool its server lists, which is known only when a call is made, and is checked then.
*Accept:* `decide: { order_prat: [sam] }` in `office.yaml` gives a reading with the house's workers running, and the reading's problems include one on the top team naming `order_prat`. With the fixer given a `github` entry, `github_create_issue: [kit]` is no problem, and `gitlab_x: [kit]` is one.

**C-office-07** Which tools a worker holds is learned from its definition as Corbie's client fills it out, tools files included, and from no reading of the file of Bulldog's own. A reading of its own would differ from what runs. *Test:* `office_tools_from_filled_definition`

**R-office-32** A team's `about` is one line saying what the team looks after, given to workers choosing whom to ask and shown on the screens; a team file may leave it out, and one that holds more than one line is a problem naming the team and `about`.
*Accept:* the garden's `about` reads as "The beds, the lawn and the shed." `about` left out of `support/team.yaml` reads, and support has no `about`. `about: |` followed by two lines in `garden/team.yaml` is a problem naming `garden` and `about`.

## People

**R-office-13** Every person is written once, under `everyone` in `office.yaml`, by a name of their own, with an optional `name`, an optional `title`, and, for each bridge that knows them, the bridge's name as a key and who they are there as its value; a person's bridges are read in the order written; a person with no `name` is called by the name they are written under.
*Accept:* the work office reads five people; `al` is called "Al", with the title "Runs the office.", and his bridges are, in order, Slack with identity `U0AL0000` and GitLab with identity `"1041"`; written `{ gitlab: "1041", slack: U0AL0000 }`, GitLab comes first. With `name:` removed from `raj`, he is called "raj". `everyone` in `garden/team.yaml` is a problem.

**R-office-14** Each key under a person, other than `name` and `title`, names a bridge of the office, an entry under `bridges` or a folder under `bridges/`; nothing for signing in is written there.
*Accept:* `kit: { name: Kit, slack: U0KIT000 }` in the house, which has no Slack bridge, is a problem naming `kit` and `slack`, and so are `kit: { name: Kit, gitlab: "1077" }` and `sam: { name: Sam, email: sam@example.com }`, the last naming `email`.

**R-office-15** A person named in no team is a person all the same.
*Accept:* in the work office `raj` is a person, in no team's `people`, and may be named in a `decide` list.

**C-office-09** Two people never share who they are on one bridge; the identities are compared exactly as written. A yes is taken from the person an identity names, so a shared one would let either answer as the other. *Test:* `office_identity_names_one_person`

**C-office-25** How a person is reached is read from their entry in the files alone, and nothing kept outside the files, in the store, on the screens or by a bridge, adds an identity to it or overrides one; their entry, their own as much as anyone's, changes only as the files do, by a change decided like any other. Changing one's own identity on a bridge is what someone who had taken over an account would do first, and a yes is taken from whoever that identity names. *Test:* `office_person_reached_only_by_entry`

## Workers

**C-office-10** A worker is sent to Corbie as its file is written, but for a model named by one word, which is replaced by the nearest team's model of that name (R-office-34); Bulldog fills nothing else into it and rewrites nothing else. What a person reads in the file, with that model, is what runs, and the screens show the model filled in. *Test:* `office_worker_sent_as_written`

**C-office-11** A worker's definition carries its file's name; one that carries another is refused, and stops only that worker. Runs are found and shown by their machine's name, and a worker whose definition carried another would be taken for that other. *Test:* `office_definition_carries_worker_name`

**R-office-16** A worker whose definition Corbie's client refuses is given no run until it is mended, and its problems are reported with its file; when it is its team's lead, its team is reported as having a lead that cannot run.
*Accept:* the house with `sparks.yaml` naming a tool with no job reads; `sparks` is reported with everything wrong in its file and is marked as not running; the fixer, which runs sparks, and the steward, which runs the fixer, are each reported once as using sparks and do not run, and the top team is reported as having a lead that cannot run; the other workers run. With `steward.yaml` broken instead, the top team is also reported as having a lead that cannot run.

**R-office-30** A worker Bulldog starts at the top, a team's lead or a routine's worker, names as `ask:` the ask Bulldog ships, by its path under `.bulldog/`, when anything it runs can be held: a job marked for approval or permissions listed under `ask_about`, in its own definition or in any machine its tools run, colleagues and machines in `tools/` alike, as far down as they go, each read from its filled definition (C-office-07). A colleague called as a tool uses the nearest ask above it unless it names the ask Bulldog ships itself; one that can also be started at the top is held to the rule for those. A worker that must name the ask and names none, and any worker that names another as its `ask:`, is reported as that worker's problem, saying to name the ask Bulldog ships, and is given no run until it is mended.
*Accept:* the house reads with the steward naming the ask and the fixer and sparks, colleagues only ever called as tools, naming none; the fixer's held `cut_power` goes to the ask the steward names. The house's `steward.yaml` with no `ask:` is reported as the steward's problem naming `ask`, since the fixer it calls holds `cut_power`, marked for approval; the top team is reported as having a lead that cannot run, and the other workers run. With `{ at: "08:00", to: fixer, say: "Anything to mend?" }` added to the house's routines, `fixer.yaml`, naming no `ask:`, is the fixer's problem naming `ask`; with `ask: { machine: ./.bulldog/ask.yaml }` added to it as well, it reads. `ask: { machine: ./tools/my-ask.yaml }` in `sparks.yaml` is sparks' problem.

**R-office-33** A team file's `models` gives models by name: each key is a name, of small letters, figures and dashes beginning with a letter, and each value is a model written as Corbie writes one, which names the secret that holds its key, where it needs one, and never holds a key; a value that is not a mapping is a problem of the team file. What a model holds is checked by Corbie in each definition it is filled into.
*Accept:* the house's `models` reads as `usual`, with Claude Code's `sonnet`, and the garden's as `usual`, with Claude Code's `opus`; the work office's as `usual`, with `claude-sonnet-5-5` and the secret `ANTHROPIC_KEY`. `models: { usual: sonnet }` and `models: { Usual: { name: sonnet, service: claude-code } }` in `office.yaml` are each a problem naming the entry. `usual: { name: sonnet, service: claude-code, address: https://x.example }` in `office.yaml` reads; the fixer, which names `usual`, is reported with Corbie's refusal of its model, and the other workers run.

**R-office-34** A model a definition writes as one word is a name, never a model of Corbie's own, and is replaced, before the definition is checked or sent, by the model of that name in the `models` of its own team, or failing that of the nearest team around it that gives the name, out to the office; a rule's definition is looked up from its team, and a bridge's machines from the office. A rule Bulldog ships names no model by one word, and nothing is filled into it.
*Accept:* in the house the fixer's `model: usual` is filled in as `{ name: sonnet, service: claude-code }`, the house's, and a gardener whose file says `model: usual` is filled in with the garden's `{ name: opus, service: claude-code }`. With `garden/beds/team.yaml` added, giving no `models`, a worker in `beds/` that says `model: usual` is filled in with the garden's. With the garden's `models` removed, the gardener's is the house's. The fixer's definition as the screens show it holds the filled model, and its file still says `model: usual`.

**C-office-24** A model named by one word that no team from the definition's own out to the office gives is a problem of that worker, rule or bridge alone, naming the word and the file: it is given no run until it is mended, and the rest of the office reads. Sent unfilled, the word would reach Corbie as a model no service offers, and the worker would fail at its start rather than when the office was read. *Test:* `office_unknown_model_name_stops_worker`

## Routines

**R-office-17** A routine has `at`, a time of day as `HH:MM` with an optional weekday from `mon` to `sun` before it; `to`, a worker of the same team; and `say`, what that worker is told; each is needed.
*Accept:* the house's routines read. Each of these in `office.yaml` is a problem naming the routine: `at: "mon thu 09:00"`, `at: "25:00"`, `to: sam`, `to: gardener`, and a routine with no `say`.

**R-office-18** A routine's time is one Corbie would take for a timer, and one it would not is a problem of its team's file.
*Accept:* `at: "18:60"` is reported as a problem of the routine, in its team's file.

**C-office-12** A time the files write, a routine's `at` among them, is written without a zone and read in the office's `zone`, or in UTC when the office names none; an `at` that names a zone of its own is a problem. Corbie reads a time with no zone as UTC, so without the office's zone a routine in London would come an hour out for half the year. *Test:* `office_routine_time_in_office_zone`

**R-office-19** The office's `zone` is a time zone by its name in the zone database.
*Accept:* `zone: Europe/London` reads; `zone: Somewhere/Else` is a problem.

## Notes

**R-office-20** A team's notes are the files in its `notes/` folder, and are read from the folder each time they are needed; a team with no `notes/` has none.
*Accept:* a file written into `garden/notes/` is in the garden's notes at the next reading of them, with no reading of the office between. The work office's `support` has no notes.

**R-office-31** A team's notes are the team's, the same in the context of every run Bulldog starts of one of its workers and in no other team's runs, and are files of the office like any other: among the office's files, in its history, and not sealed as what a team read is. A worker has the notes tool Bulldog ships, `write_notes` in `tools/notes/notes.yaml`, only when its own definition names that file; writing a team's notes is the one write to the office's files that is not a decided change, and is still a commit with who and why.
*Accept:* with `garden/notes/beds.md` written, the notes in the context of the gardener's run on a request to the garden and of the gardener's routine run are the same and hold it, those in a steward's run do not, and `garden/notes/beds.md` is among the office's files. Bins, whose definition names no `tools/notes/notes.yaml`, is given the house's notes in its routine's runs and has no notes tool, so cannot write one.

## Bridges in the files

**R-office-21** The top team's `bridges` names each bridge the office uses, by its name, with its settings; a bridge runs the folder Bulldog ships that its `from` names, which is only one under `.bulldog/bridges/`, or else its own folder, always `bridges/<name>`, holding `in.yaml` and `out.yaml`. A `bridges` entry with no `from` and no folder at `bridges/<name>`, a `from` that names no folder Bulldog ships under `.bulldog/bridges/`, an entry with `from` and a folder at `bridges/<name>` too, a folder under `bridges/` that `bridges` does not name, a folder missing either machine, and an in machine that takes both `cursor` and `delivery` or neither (R-bridges-04) are each a problem of that bridge, shown on the office and naming the bridge; that bridge is not run until it is mended, and the rest of the office reads and runs.
*Accept:* the work office reads two bridges, `slack` and `gitlab`, with the settings written, each running the folder Bulldog ships that its `from` names; written under `lab`, the same entry is a bridge `lab` running `.bulldog/bridges/gitlab`. A `from` of `./bridges/gitlab`, `./.bulldog/bridges/matrix` or `./.bulldog/tools/talk`, and a folder `bridges/gitlab/` beside `gitlab`'s `from`, each give a problem on the office naming `gitlab`, the last saying `bridges/gitlab is not run: gitlab has a from`; with `./bridges/gitlab` as the `from` and the folder there too, there are both problems, and neither says the bridge runs `./bridges/gitlab`. With `gitlab` copied into `bridges/gitlab/` and its `from` taken out, it runs that folder; removing `gitlab` from `bridges`, removing `bridges/gitlab/`, removing `bridges/gitlab/out.yaml`, and writing `bridges/gitlab/in.yaml` to take both `cursor` and `delivery`, and then neither, each give a reading with a problem on the office naming `gitlab`, no run of `gitlab`, and `slack` and the office's workers running; so does adding `bridges/jira/` with files in it and nothing named for it, naming `jira`; an empty folder is not among the office's files. A person's `gitlab` identity left when `gitlab` is removed from `bridges` is part of that bridge's problem, not the person's. The house without its `telegram` entry and its people's Telegram identities reads with no problem.

**R-office-22** What a bridge's settings hold is the bridge's own, but for the folder its `from` names (R-office-21) and the places it listens in, whose shape is the bridges page's: the office checks that the settings are a mapping and that each team a place names is a team of the office, and either failing is a problem of that bridge, which stops only that bridge (C-office-15).
*Accept:* `slack: { from: ./.bulldog/bridges/slack, places: { "#help": { guests: true } }, anything: 3 }` reads, and its settings, `from` among them, are given back as written. `slack: "#help"` is a problem on the office naming `slack`, and `slack` is not run. `"#help": { team: billing }` under the Slack bridge's places, with no team `billing`, is a problem on the office naming `slack` and `billing`; `slack` is not run, and `gitlab` and the office's workers run.

**R-office-23** A bridge whose `in.yaml` or `out.yaml` Corbie's client refuses carries nothing until it is mended, and is reported with its file; the rest of the office reads.
*Accept:* the work office with its GitLab bridge copied into `bridges/gitlab/`, its `from` taken out, and a broken `bridges/gitlab/in.yaml` reads, `gitlab` is reported on the office as not running with what is wrong in its file, and `slack` runs.

## Reading

**R-office-24** The office is read whole: every team file, every worker's and bridge's definition, and every rule, each checked by Corbie's client as it would be before a start.
*Accept:* a reading of the house as the overview shows it, with its Telegram bridge, makes one check of each of its eight workers, each with its one-word model filled in, its bridge's two machines and its rule, `small-orders.js`, found as the bare script it is in its team's folder, which is all Corbie checks of a script before it runs, and of no other file. A reading of the work office checks `payments/no-account-numbers.yaml` as a definition.

**C-office-13** Every problem of a reading is reported together, each with its file, where in the file when that is known, and what is wrong. A person mending the files should see everything wrong at once, not one thing per attempt. *Test:* `office_every_problem_together`

**C-office-14** Reading and checking the office need no running Corbie, no network and no model. A person checks an office before serving it, and the tests check offices with none of these. *Test:* `office_checks_offline`

**R-office-25** A problem in a team file refuses the reading, and the office goes on as it was last read well.
*Accept:* with the house read and running, a second `steward` written into `garden/` gives a reading that is refused. The workers, teams and routines in use are those of the first reading, and the problem is reported.

**C-office-15** A definition Corbie refuses stops only itself and the workers whose tools run it, each of those reported once with the one it runs that is refused, whether it is a worker's, a bridge's or a rule's, and a bridge that cannot be used, whatever is wrong with it, stops only that bridge; neither refuses the reading. One bad file should not take the office down. *Test:* `office_broken_definition_stops_itself`

**R-office-26** The problems reported are always those of the latest reading, and a problem that is mended is no longer reported.
*Accept:* a reading with a broken `sparks.yaml` reports it. Once the file is mended, the next reading reports no problem.

**R-office-27** Until the office has been read well once since the server started, it starts nothing and only reports its problems; no reading is kept from one start to the next.
*Accept:* serving a copy of the house whose `office.yaml` names no `lead` starts no run, and its problems are reported. Writing `lead: steward` into it gives a good reading, after which the office's runs start. With the house served and read well, the server stopped, `lead` removed by hand and the server started again, no run starts and the problem is reported.

**R-office-28** The office is read again whenever any of its files is written, added, removed or renamed, and a change to a definition reaches its worker at its next run, while a run under way goes on as it was started.
*Accept:* with the house running, a change to `fixer.yaml`'s `system` is in the definition of the fixer's next run, and a fixer run already under way keeps the definition it started with.

**C-office-16** A rename, and a write that leaves a file's time older than before, are noticed as changes. Neither changes how many files there are or the newest time among them, so a look at those alone misses both. *Test:* `office_rename_noticed`

**C-office-17** A good reading replaces the last one whole: nothing ever goes by half of one reading and half of another. A run started while the files change must see one office. *Test:* `office_reading_replaced_whole`

## Names

**R-office-29** The names of people, workers, teams and bridges are small letters, figures and dashes, beginning with a letter.
*Accept:* a person `Sam`, a worker file `Fixer.yaml`, a team folder `the garden/` and a bridge `2way` are each a problem naming the name.

**C-office-18** A name is used once in the whole office, among people, workers and teams together; a model's name is not among them, and a team within may give it again. Deciders, leads and requests name each by its name alone. *Test:* `office_names_used_once`

**C-office-19** These names are reserved, and a person, worker, team or bridge so named is a problem: `office`, `team`, `notes`, `bridges`, `chat`, `routine`, `bulldog` and `guest`; and no bridge is called `name` or `title`. Each is a file or folder of the office, the built-in chat, a key of a person, or who Bulldog itself writes as, and one so named would be taken for it. The built-in chat is never written under `bridges`. *Test:* `office_reserved_names_refused`

## Interfaces

- **Requests** read each team's people, each person's identity on each bridge, each team's lead and the team each bridge's place names (R-office-22), so that a request heard at a listed place goes first to that team's lead, its origin where in that place it was said, and say who stands as the people of a team with none (R-requests-36): its own, or those its decisions reach.
- **Work** starts workers from their definitions as read, each with its one-word model filled in (R-office-34), keeps to `runs`, starts each routine's clock with the office's zone, gives workers each team's `about` (R-office-32), gives every run it starts of a team's worker the team's notes, ships the notes tool, which a worker has only when its definition names its file (R-office-31), and is told when a reading replaces the last.
- **Decisions** reads the `decide` and `disclosure` lists and the notes a disclosure check reads, and runs the rules they name, each as written (C-office-23). The office gives the people a team's decisions reach (R-decisions-39): those its `everything` names, or its people where it writes none, or, where that names nobody, those of the first `everything` outward that does, with anyone no longer in the office left out; Decisions and Requests both take them from here.
- **Bridges** reads the `bridges` settings, the folders under `bridges/` and the folders Bulldog ships that a `from` names, says the shape of a bridge's places, whose teams the office checks (R-office-22), is told of a broken bridge, which the office reports and stops alone (R-office-21), and matches a bridge's identity to a person.
- **Changes** checks a change by a reading of the folder as it would be after it, writes a person's entry only by a change decided like any other (C-office-25), and keeps each note a worker writes as a commit.
- **Server** gives the folder at each start, writes the house without its bridge into an empty folder (R-server-06), holds the office's identity, sends a one-time link over a person's bridges in the order written (R-office-13), and shows each team's `about`, notes and problems of the latest reading, and each worker's model as filled in (C-office-10), to those who may see them.
- **Corbie's client** checks every definition and rule, as it does before a start (R-office-24).
