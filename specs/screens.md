# Screens

Bulldog serves three screens to the office's people, and the API behind them. A person signs in with a key, and each screen shows only what that person may see.

## Signing in

A person's key is the office secret's signature over their id and a hash of their entry in `everyone`. A person signs in with a link, which opens a page whose button signs them in:

| Link | Where it comes from |
|---|---|
| a key link | `corbie bulldog key <directory> <person>` prints it; it holds the person's key |
| a sign-in link | the sign-in page sends it; it is good for 15 minutes, in one browser |

The sign-in page works like this:

1. The person gives their name. The page sets a random cookie in the browser, and answers the same, and at once, whatever the name.
2. Bulldog sends them a link, signed over that cookie, in their direct messages on the first bridge in `bridges:` that can message them directly.
3. The person opens the link in the same browser and presses the button on its page. That press, a POST to the link's own address, signs them in.

- Opening a link signs nobody in, since a service may open it to show a preview.
- A person is sent at most one link in each minute.
- The name given is a person's id in `everyone` or their `name`, in any case.
- The key is kept only in an HttpOnly, SameSite=Strict cookie, Secure when the screens are reached at an `https://` address, and compared in constant time.
- Changing or removing a person's entry signs them out. Replacing the office's secret signs everyone out.

## Who sees what

- A person of a request's team sees it whole. A team with no people has the office's people as its people.
- The person who asked a request sees it sealed, and so do the people of the team that asked a sub-request, and the people a question or a decision of it waits on.
- Sealed, a request shows its source, what was said there, whom it waits on, the outcome as sent, and each fork as who, when and how it ended.
- Questions, answers, notes, held jobs, the lead's conversation and every stage's output are what the team read. They are drawn hatched, and shown only to its people.
- A question or a decision is shown, with its notes and the held jobs, to the people it is put to: those it waits on, and the one who answered it. A question Bulldog refused is put to no one.
- A request one may not see is answered as one that is not there.
- Every person sees every team's page and the office's. A team's docs are shown only to its people.
- The person who asked a request, and its team's people, may say more in its thread and stop it.

## The look

The screens are paper and ink, with plan blue for structure and work, wait orange for what waits on someone, hatched areas for what is sealed, and teams drawn as rooms with walls. Every colour and size is a token of `style.css`: `--paper`, `--sheet`, `--ink`, `--ink2`, `--ink3`, `--line`, `--faint`, `--plan`, `--plan-soft`, `--plan-deep`, `--wait`, `--wait-soft`, `--bad`, `--good`, `--hatch-a`, `--hatch-b` and `--wall`, set in IBM Plex Sans and IBM Plex Mono. A rule is named by its file's name without its ending, such as `small-orders`.

## Requests

The home screen, from the top:

1. A box to ask any team: choose the team, say something, attach files.
2. The questions and decisions waiting on you, as cards with their choices. A decision's card shows its question, then its held jobs, each with what it does and what it was given, and what each decider before you said.
3. The requests you may see, newest first, at most 200. Each row has its flow as small dots on a line, its team, who asked, its first line, and when.

## A request

- The team is a room: a box with a 2px `--ink` wall, square corners and `--sheet` inside, with the team's name on the top wall.
- One 2px line runs left to right. SOURCE sits on the left wall and REPLY on the right wall, and HEED, WORK and CHECK are inside. A stage the team does not have, or that did not run, is not drawn.
- SOURCE, WORK and REPLY are 12px circles. HEED and CHECK are 12px diamonds.
- Under each node is its word in 11px mono capitals, then one mono line: who asked and where; the decider that answered and its word; the lead and the time it took; sent or not, with the error to the team's people.
- A node not reached has a `--paper` fill and a `--line` border. A node passed has a `--plan` border, with the line behind it `--plan`.
- A working node is filled `--plan` and breathes. A waiting node is filled `--wait` and rings.
- A heed's no is dashed `--ink3`, and the line ends there. A check's no is hollow `--bad`, with a dashed `--bad` line to REPLY. A failed node is a `--bad` square.
- A stage running when the request was stopped is filled `--ink3`, with `stopped` under it.
- Forks drop from the bottom of WORK through a gap in the bottom wall. Each is a plain 1.5px `--ink3` line ending in a row: a chip (TEAM, PERSON or DECISION), the party, the question and the answer, and a status dot.
- A DECISION row is a decision on held jobs: the jobs, each with what it does and what it was given, who decided, the answer, and folded, what each decider said. One that said no has a hollow `--bad` dot.
- To those outside the team, a fork's row shows who, when and how it ended, and a decision that came to no person is not shown. A waiting fork is `--wait`, and when it waits on the viewer, its answer card sits under the row. A TEAM row links to that team's request when the viewer may see it.
- A team whose lead is a person shows the work's wait as a PERSON fork. A question that a heed or a check puts to people is no fork: it is a card on the Requests screen, and under the room on this one.
- For the team's people, the tools and colleagues the lead used are a small tree under WORK, and a folded hatched panel, "what the team read", follows the run as it goes. Others see that area hatched and empty.
- Each call in the tree is marked done or failed, and a call with no result is marked working while the request runs and stopped after it ends.
- Below the room is the thread: every request in it as rows, this one marked, each linking to its request, and a box to say more.
- Under 640px the line runs downward, and the forks indent under WORK.

## A team

A team's page draws the same room with no run. Each node shows what is written for it:

| Node | What it shows |
|---|---|
| SOURCE | the places that ask the team, each with its bridge and whether guests may ask, then the screens and other teams |
| HEED | the heed's deciders |
| WORK | the lead, its colleagues each with what it uses, and the tools the office writes, those held for approval marked |
| CHECK | the check's deciders |
| a fork line from WORK | `decide`, with its deciders, or the team's people without one |

Beside the room are the team's people; its workers, each with its definition as sent to Corbie; its routines, with their next time; its docs, which its people read and edit; its problems; and its request machine as sent to Corbie.

The office's page draws the office's room with its teams inside as rooms. It shows each bridge with its in run running, failing or stopped, and Corbie's error when it fails.

A team's people read and edit its docs on its page. Bulldog does it through a run of `./.bulldog/docs/docs.yaml`, labelled `kind=docs` and started with the team's start list (Office), so Bulldog holds no key to kept values.

## The API

The screens use this API, JSON in and out. A request is named by its run's name after the client's name, such as `20261022T100204.000Z-3c9e1a0b7d`. Times are RFC 3339 in UTC, and the screens show them in the office's zone.

| Request | What it gives or does |
|---|---|
| `GET /api/me` | who is looking, with their teams, and the office's name and zone |
| `GET /api/requests` | the questions and decisions waiting on the one looking, oldest first, and the requests they may see, newest first |
| `POST /api/requests` | asks a team in the chat, `{ team, said, files }`, each file `{ name, data }` with `data` in base64 |
| `GET /api/requests/{id}` | one request, as the one looking may see it |
| `GET /api/requests/{id}/files/{n}` | a file said with the request |
| `POST /api/requests/{id}/say` | says more in the request's thread, `{ said }`: a new request of the same team |
| `POST /api/requests/{id}/stop` | stops the request and everything below it |
| `POST /api/waits/{ref}/answer` | answers a question or a decision waiting on the one looking, `{ answer }`; a decision takes one of its choices |
| `GET /api/office` | the office: its teams as rooms, its people, its bridges and its problems |
| `GET /api/teams/{team}` | one team, `office` among them, as its page shows it; its docs only to its people |
| `GET /api/teams/{team}/docs/{name}` | one doc of the team, to its people |
| `PUT /api/teams/{team}/docs/{name}` | writes one doc of the team, `{ text }`, by its people |
| `GET /api/events` | a stream of `changed` events: `{ request }` for each request that changes, of those the one looking may see, and `{ office: true }` when the office is read again or a bridge's in run changes |

These need no key, and answer pages:

| Request | What it gives or does |
|---|---|
| `GET /signin` | the sign-in page: a box for one's name |
| `POST /signin` | sends a sign-in link to the person a form's `name` names; answered the same whatever the name |
| `GET /signin/link?p=<person>&t=<time>&s=<signature>` | the sign-in link's page, with its button; signs nobody in |
| `POST /signin/link?p=<person>&t=<time>&s=<signature>` | signs the person in when the browser's cookie is the one the link was signed over, within 15 minutes |
| `GET /key?p=<person>&k=<key>` | the key link's page, with its button; signs nobody in |
| `POST /key?p=<person>&k=<key>` | signs the person in while the key is theirs |
| `POST /signout` | takes the key from the browser |

- An answer of the API that is not a success is JSON, `{ error }`, with a sentence a person can read. Without a key, it is 401.
- A request, a doc or a file the one looking may not see is answered 404, with the same sentence as one that is not there.
- A POST or a PUT that a page of another site sends is answered 403, so that no other site acts as the one signed in, or signs a browser in as someone else.
- A file is served with its type told from what it holds, never from its name, and with a sandbox that lets it run nothing. Only a picture other than SVG, or a PDF, is shown; anything else is downloaded.
- The screens load nothing from any other site, and every answer is served with the policy `default-src 'self'`.
