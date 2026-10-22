# Bulldog

An office of teams on Corbie (`../corbie`): people and workers in teams, asked things through bridges, with decisions that wait for the right yes. Each request is one Corbie run of its team's request machine. Corbie runs and keeps everything; Bulldog keeps nothing but the office's id and secret.

## Where to start

- `specs/README.md`: the whole of it in a page, the words, and the map of the other pages. Each area has an overview page and a rules page (`specs/rules/<area>.md`) of requirements `R-<area>-nn` and constraints `C-<area>-nn`. The specs describe how things are now, and are the authority.
- Each area is one package: `internal/office`, `internal/requests`, `internal/bridges` and `internal/screens`. Beside them are `internal/testkit`, `shipped` (what Bulldog ships, served at `./.bulldog/`), the program in `cmd/corbie-bulldog`, `cmd/demo`, `cmd/internal/serve` (serving an office, for both programs) and `examples` (the two offices, embedded).
- `examples/house`: an office, as a person writes one. `examples/work`: one Slack bot over many teams.
- `go.mod` builds against the Corbie beside it, `../corbie`.
- The Bulldog before this design is in the history of `main`. It is a reference for how something was done, never a source to copy from.

## What holds

- **Six concepts**: team, person, worker, bridge, request, decision. Add no other concept a person must learn.
- **One machine per team, one run per request.** A team file is a short form of its request machine, a Corbie flow with the stages `heed`, `work` and `check`, and Bulldog fills it out. A worker is sent to Corbie as it is written, but for a model named by one word.
- **Inside a team, workers use each other as Corbie tools. Between teams, work goes as a request.**
- **Bulldog keeps nothing but the office's id and secret.** Every request, question, routine and bridge is a Corbie run, found again in the store after a restart. A team's docs are values kept in Corbie's store. There is no database, and doing a thing twice changes nothing.
- **Bulldog drives, and nothing in Corbie calls Bulldog.** Bulldog starts runs and answers their tickets. A question to a person or a team is a run that waits.
- **Teams are sealed**: what a team read is seen only by its own people. Bulldog carries only a team's answers and its questions out of it. Its answers, and what it asks with `ask_team`, pass its `check`.
- **Every way in and out is a bridge**, a directory of Corbie definitions. Bulldog talks to no outside service itself, and its own screens are the one way in that is not a bridge.
- **Every run Bulldog starts carries labels**, `bulldog=<office id>` and `kind` among them, and is found by them. Labels and run names hold no words of a request and no plain id of a person.
- Corbie's ideals come first. A workaround names what Corbie lacks: add it to the list in `specs/README.md`.

The code goals are Corbie's, in the same order: simple, one path, obvious logic, errors reported where a person will read them, data flowing through functions. Everything a reader sees, the specs, the shipped pages and what Bulldog prints, follows the writing rules in `../corbie/CLAUDE.md`.

## Working on it

- Go through mise: `mise exec -- go …`, or plain `go` with `GOWORK=off` where mise is not installed. `mise run check` vets and tests with the race detector.
- Tests need no network, no key and no model: they run a whole Corbie in the test's process with a stand-in model (`internal/testkit`), move a clock the test controls, and never wait a fixed time.
- Every test begins `t.Parallel()`, but one that sets an environment variable or the directory, and shares nothing with another but what is safe to share.
- A constraint's test is named as its rule says (`C-requests-42 … Test: requests_started_once` is `TestRequestsStartedOnce`).
- Until version 1, nothing is kept backwards compatible.
- A behaviour change updates its spec in the same commit. No references to the specs in code comments: the link lives in commit messages.
- Commits go straight to `main`, and the message starts with the area and names the IDs: `requests: a question to another team passes the team's check (C-requests-36)`.
