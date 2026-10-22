# Office

The rules behind the [Office overview](../office.md). Area code: `office`. The house and the work office are `examples/house` and `examples/work`; a test writes one, or a copy changed as the rule says, into a directory of its own and reads it.

## The directory

**R-office-42** The office is the directory Bulldog is given, its top team's file `office.yaml` at its top; a team within is a directory directly inside a team's directory that holds `team.yaml`.
*Accept:* the house reads two teams, `office` and `garden`. With `garden/beds/team.yaml` added, it reads three, `beds` within `garden`. A directory with no `office.yaml` is refused, and the refusal says so.

**R-office-43** A worker is a Corbie definition, an agent, a flow or a statechart, directly in a team's directory, named by its file's name; its definition's name is the same, and a definition anywhere else is not a worker.
*Accept:* the house reads `steward`, `fixer` and `clerk` as the office's workers and `gardener` as the garden's. `small-orders.js` and a flow at `tools/tidy.yaml` are not workers. A `fixer.yaml` whose definition says `agent: mender` is a problem that names the file and both names.

**R-office-44** `./.bulldog/<path>`, written from any directory of the office, names what Bulldog ships, and Bulldog serves it from its binary.
*Accept:* payments' `heed: [./.bulldog/rules/mentioned/mentioned.js]` and the ledger's `tools_file: [./.bulldog/tools/talk/talk.yaml]` read the shipped files. `./.bulldog/rules/nothing.js` is a problem that names the path.

**C-office-30** Bulldog's loader reads only the office's files and what Bulldog ships, and a path that leaves the office directory is a problem. A definition read from outside would run with nobody seeing it in the office. *Test:* `office_nothing_read_outside`

## A team file

**R-office-45** A team file takes `name`, `about`, `people`, `lead`, `heed`, `check`, `decide`, `routines` and `models`, and `office.yaml` takes these and `zone`, `everyone` and `bridges`.
*Accept:* the house and the work office read with no problem. `budget: 40` in `garden/team.yaml` is a problem that names the key and the file, and so is `zone` there.

**C-office-31** A key a team file does not take, or one only `office.yaml` takes written in another team file, is a problem, never read past. A key read past is a setting a person believes is in force and is not. *Test:* `office_unknown_key_refused`

**R-office-46** Every team has a `lead`: a worker of the team, or a person.
*Accept:* the house's `lead: steward` reads, and so does `lead: sam` in `garden/team.yaml`. A team file with no `lead` is a problem, and so is `lead: gardener` in `office.yaml`.

**R-office-47** A worker's `model:` of one word is filled in from the nearest team's `models`, its own team's first and the office's last, and a word no team gives is a problem of the worker.
*Accept:* the steward's `model: usual` is sent to Corbie as `{ name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }`, and the gardener's as the garden's `claude-opus-5-5`. `model: fast` is a problem that names the worker.

**R-office-48** A routine is `{ at, say }`, where `at` is a time of day, optionally after a weekday, in the office's `zone`, or UTC without one.
*Accept:* `"mon 09:00"` and `"18:00"` read. `"25:00"` is a problem of the team file, and `zone: Europe/Londn` a problem of `office.yaml`.

## Deciders

**R-office-49** A decider is a rule, named by its path to a script or to a definition Bulldog ships; a worker of any team, named by its name; or a person, named by their name. People may only end a list.
*Accept:* payments' `check: [./no-account-numbers.js, lee, priya]` and the house's `decide: [./small-orders.js, clerk, sam]` read. `[lee, ./no-account-numbers.js]` is a problem, and so is `[./missing.js]`.

**R-office-50** A worker named as a decider declares `reply: { says: string, note: "string?" }`.
*Accept:* the clerk, which declares it, reads as a decider. The steward named in `decide` is a problem that names the steward.

**C-office-32** A worker or a rule named as a decider that holds a job for approval, or lists `ask_about`, is a problem of the team file that names it. Corbie refuses a machine run as an ask that asks for approval, so one such decider would stop the whole team. *Test:* `office_decider_never_asks_approval`

## People

**R-office-51** A person is written once in `everyone`, with `name` and, under each bridge's name, their lasting id on it; a team's `people`, its deciders and its `lead` name people by their key.
*Accept:* the house reads Sam with `telegram: "4471"`. `people: [sam, jo]`, with no `jo` in `everyone`, is a problem that names `jo`.

**C-office-33** One identity on one bridge names one person, compared exactly as written. An answer is taken from the person an identity names, so a shared identity would let either person answer as the other. *Test:* `office_identity_names_one_person`

**R-office-52** A team with no people has the office's people as its people, for being asked and for seeing its requests.
*Accept:* in the work office, platform's people are Al, and a held job of platform's ops is put to Al.

## Names

**R-office-53** Names of people, workers and teams start with a small letter and hold small letters, figures and dashes; each is used once in the office, and `office`, `team`, `bulldog`, `chat`, `guest`, `wait`, `ask` and `clock` are reserved.
*Accept:* a team directory `2nd-floor` is a problem. So is a worker `sam.yaml` in an office with a person `sam`, and a person named `chat`.

## Reading the office

**R-office-54** Bulldog reads the office again when any file in it changes. A problem in `office.yaml` or a team file keeps the last good reading in force, a broken worker, rule or bridge stops only itself and the teams that use it, and every problem is shown.
*Accept:* with the house served, `budget: 40` written into `garden/team.yaml` leaves both teams as they were and shows the problem. A `fixer.yaml` that is not YAML stops the office's requests and shows the problem, and the garden still takes requests.

**R-office-55** `corbie bulldog check <directory>` reads the office as serving does, prints every problem with its file, starts nothing, and exits with a failure status when there is any problem.
*Accept:* on the house it prints nothing and exits 0. With `budget: 40` in `garden/team.yaml` and `people: [sam, jo]` in `office.yaml`, it prints both problems, each naming its file, and exits 1. No run is started either time.

**C-office-34** Every problem of a reading is found and reported together, never only the first. One problem at a time costs a person a round of editing for each. *Test:* `office_every_problem_together`

## What Bulldog ships

**R-office-56** Bulldog ships, under `./.bulldog/`, `wait/wait.yaml`, `clock/clock.yaml`, `docs/docs.yaml`, `tools/talk/talk.yaml`, `tools/docs/docs.yaml`, `rules/mentioned/mentioned.js`, `rules/for-us/for-us.yaml` and the bridges `slack`, `telegram`, `gitlab` and `jira`, and each shipped directory has a `README.md`.
*Accept:* each path reads through the loader, Corbie's parser takes each definition, and each shipped directory holds a `README.md`.

## Docs

**R-office-57** A team's docs are values kept in Corbie's store at `<office id>/<team>/docs/<name>`, and a doc's name is one part of a kept path.
*Accept:* the ledger's `write_doc("refunds.md", …)`, once approved, keeps the text at `<office id>/payments/docs/refunds.md`, and `read_doc` with no name then lists `refunds.md`. `write_doc("Refunds.MD", …)` fails, and its error gives the rule for a name.

**R-office-58** `read_doc(name)` reads one of the team's docs, or lists them with no name, and is allowed `kept.read` only; `write_doc(name, text)` writes one whole, is allowed `kept.write` only, and is marked for approval.
*Accept:* the ledger reads `refunds.md` back as it was written. `read_doc`'s job lists no `kept.write` entry, and `write_doc`'s lists no `kept.read` entry.

**C-office-35** Every run Bulldog starts for a team, a request or a docs run, is started with the list `network`, `disk`, `exec`, `env`, `secret:*@*`, `kept.read:<office id>/<team>/**` and `kept.write:<office id>/<team>/**`. The shipped docs tools take their path from the worker's input, which a calling model may write, so only the start list keeps a team's runs to its own docs. *Test:* `office_team_runs_keep_to_own_docs`

**C-office-36** `write_doc` waits for the team's `decide`, which is shown the whole new text. Anything a bridge hears can steer a lead, and every later run reads a doc as the team's own. *Test:* `office_write_doc_waits_for_decide`

## The office's id and secret

**R-office-59** Bulldog writes `.bulldog/id`, a random id of 16 hex digits, once, at the first start; it is the one file Bulldog writes in the office.
*Accept:* serving the house writes `.bulldog/id`. Serving it again keeps the same id, and no other file of the office changes.

**C-office-37** The office's secret is made at the first start and kept in Bulldog's own directory in the user's config directory, never in the office directory. The office directory is edited, copied and put in git, and the secret signs every key. *Test:* `office_secret_never_in_office`

## Interfaces

Uses:

- **Corbie.** Its parser, to check each worker and rule through a loader that serves the office's files and `./.bulldog/`; its rule for a kept path, for a doc's name.
- **Shipped.** The files served at `./.bulldog/`.

Used by:

- **Requests.** The reading: teams, people, workers, deciders, routines, models and the loader; the start list of a team's runs; the office's id and secret.
- **Bridges.** The bridges with their settings, and each person's identities.
- **Screens.** The teams, people, problems and docs' place; the start list; the office's secret.
