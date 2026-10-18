# Bulldog

An office of teams on Corbie (`../corbie`): people and workers in teams, asked things through bridges, with decisions that wait for the right yes. Bulldog keeps what was asked, what was done and who decided; Corbie runs everything.

## Where to start

- `specs/README.md`: the whole of it in a page, and the map of the other pages. Each area has an overview page and a rules page (`specs/rules/<area>.md`) of requirements `R-<area>-nn` and constraints `C-<area>-nn`. The specs describe how things are now, and are the authority.
- Each area is one module: `internal/<area>`, and the program in `cmd/corbie-bulldog` for the server area.
- `examples/house`: an office, as a person writes one. `examples/work`: one Slack bot over many teams.
- The old app is at `../corbie-bulldog`. It is a reference for how something was done, never a source to copy from.

## What holds

- **Six concepts**: team, person, worker, bridge, request, decision. Add no other concept a person must learn.
- **Inside a team, workers use each other as Corbie tools. Between teams, work goes as a request.**
- **The files are the office.** A worker is sent to Corbie as it is written, but for a model named by one word. Bulldog writes no definition.
- **Bulldog drives, and nothing in Corbie calls Bulldog.** Only `internal/work` talks to Corbie. A tool is a run that waits.
- **Teams are sealed**: what a team read is seen only by its own people. What was done is seen by the asker and wherever it was asked.
- **Bulldog speaks to no outside service.** Every way in and out is a bridge, a folder of Corbie definitions.
- **Every run Bulldog starts carries labels**, and is found by them.
- Corbie's ideals come first. A workaround names what Corbie lacks: add it to the list in `specs/README.md`.

The code goals are Corbie's, in the same order: simple, one path, obvious logic, errors reported where a person will read them, data flowing through functions.

## Working on it

- Go through mise: `mise exec -- go …`. `mise run check` vets and tests with the race detector. Tests need no network, no key and no model: they run a whole Corbie in the test's process with a stand-in model (`internal/testkit`), move a clock the test controls, and never wait a fixed time. Every test begins `t.Parallel()`, but one that sets an environment variable or the directory, and shares nothing with another but what is safe to share.
- A constraint's test is named as its rule says (`C-work-12 … Test: work_kept_answer_given_again` is `TestWorkKeptAnswerGivenAgain`).
- Until version 1, nothing is kept backwards compatible.
- A behaviour change updates its spec in the same commit. No references to the specs in code comments: the link lives in commit messages.
- Commits go straight to `main`, the message starts with the area and names the IDs: `work: a kept answer is given again after a restart (C-work-12)`.
