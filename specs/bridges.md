# Bridges

A bridge joins the office to an outside service. It is a directory of two Corbie definitions: `in.yaml`, which hears, and `out.yaml`, which sends one message.

## A bridge in the office

```yaml
# office.yaml, in the work office
bridges:
  slack:
    from: ./.bulldog/bridges/slack
    places:
      "#help": { guests: true }        # anyone in #help may ask the office
      "#refunds": { team: payments }   # people in everyone ask payments
      direct: {}                       # people ask the office in a direct message
```

- `from` names the bridge's directory: one Bulldog ships, such as `./.bulldog/bridges/slack`, or the office's own, such as `./bridges/pager`.
- The bridge's settings are its entry but `from`, and its in and out are given them.
- `places` lists where requests are taken. Each place names the team asked there, the office without one, and whether `guests` may ask.
- `direct` is any person's direct messages to the bot.
- The bridge's name is the key that people's identities on it are written under in `everyone`.

## In

`in.yaml` is a statechart that runs for as long as the bridge is on, with `keep: 12h` and the input `{ settings, cursor }`.

1. A job hears for up to 5 minutes, and emits `heard`, `{ things: [...], cursor }`, for what it hears.
2. When the job ends, the chart hears again, from the cursor the job returned.
3. When the job fails, the chart rests 30 seconds, then hears again.

```yaml
statechart: telegram-in
keep: 12h
input: { settings: map, cursor: any }
initial: hearing
states:
  hearing:
    job:
      script: ./telegram.js
      input: { settings: "${input.settings}", cursor: "${states.hearing.output?.cursor ?? input.cursor}" }
      timeout: 6m
      allow: [net:api.telegram.org, "secret:TELEGRAM_TOKEN@net:api.telegram.org"]
    on: { done: hearing, failed: resting }
  resting:
    timers: { again: 30s }
    on: { again: hearing }
```

Bulldog keeps one in run for each bridge, labelled `made=<fingerprint>` of the bridge's definitions and settings. When the fingerprint changes, or the run ends, Bulldog starts a new one, given the last cursor the old one emitted, and cancels the old. A new bridge's first in run is given no cursor.

## A thing heard

A thing is `{ id, at, kind, from, place, where, text, files, mentioned, answers, ref, choice }`:

| Field | What it is |
|---|---|
| `id` | the service's lasting id for the thing |
| `at` | when the service says it was said |
| `kind` | `said`, or `press` of a button |
| `from` | the sender's lasting id on the service, never a handle they can change |
| `place` | the listed place it was heard at, as the settings name it, or `direct` |
| `where` | where it was said, as narrow as the service allows, such as a thread; an out sends there |
| `text`, `files` | what was said; the files as blobs |
| `mentioned` | the message names the bridge's bot |
| `answers` | the id of the message it replies to, if any |
| `ref`, `choice` | for a press: the out message's `ref`, and the choice pressed |

A shipped in hands over nothing its own bot or another bot sent.

## What Bulldog does with a thing

Bulldog reads what each in run emits, and acts on each thing in this order:

1. A press answers the wait whose `ref` it carries.
2. Something said at `direct` whose `answers` is the `sent` of a carried question answers that question. One that replies to nothing answers the latest question carried to its sender on that bridge before it was heard, while that question still waits.
3. Something said at a listed place starts a request of the place's team. Its asker is the person whose entry gives that identity on that bridge, or a guest where the place lets guests ask.
4. Anything else is let go.

Requests answers a wait only when its rules for who may answer hold.

Each thing is taken once:

- A request heard is named `<client>/<stamp of at>-<keyed hash of the bridge and id>`. Bulldog looks for that name first, and starts only when there is no such run.
- A thing that answered a wait is known from the wait's answer, which holds the thing's id.
- A thing more than a day old when Bulldog reads it is let go.

## Out

`out.yaml` carries one message. Its input is `{ settings, to, direct, text, choices, ref, answers }`, and it gives `{ sent }`, the id of the message it sent, or throws.

| Input | What it is |
|---|---|
| `settings` | the bridge's settings |
| `to` | the `where` to send to, or, with `direct`, a person's lasting id |
| `direct` | `true` when `to` is a person |
| `text` | the message |
| `choices` | up to 8 short answers, offered as buttons |
| `ref` | for a question, a short keyed hash of the wait run, which a press gives back |
| `answers` | the id of the thing the message replies to |

- An out whose input does not take `direct` sends to no person directly.
- A question, a decision and a sign-in link go to a person only in their direct messages, never to a channel. A person no bridge can message directly is asked on the screens alone, and is sent no sign-in link.
- The ticket's value never leaves Bulldog. A message carries `ref`, which names the wait run and nothing else.
- Each out run has the labels `kind=out` and `bridge=<name>`, and is named from what it carries: the request it answers, or the wait and the person it asks. So a message is sent once.
- An out run that fails is shown on its request with Corbie's error, and is not tried again.

## The bridges Bulldog ships

| Bridge | Hears by | Sends to a person directly |
|---|---|---|
| `slack` | Socket Mode | yes |
| `telegram` | long poll | yes |
| `gitlab` | poll | no |
| `jira` | poll | no |

Each is a directory under `./.bulldog/bridges/`, with a `README.md` that says what the service needs, the settings, and what the bridge leaves out.
