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

- `from` names the bridge's directory: one Bulldog ships, such as `./.bulldog/bridges/slack`, or the office's own. Without `from`, it is `./bridges/<name>`.
- The bridge's settings are its entry but `from`, and its in and out are given them.
- `places` lists where requests are taken. Each place names the team asked there, the office without one, and whether `guests` may ask.
- `direct` is any person's direct messages to the bot.
- The bridge's name is the key that people's identities on it are written under in `everyone`.
- Bulldog checks `in.yaml` and `out.yaml` with Corbie's parser when it reads the office. What the parser refuses is a problem of that file, and stops the bridge alone.

## In

`in.yaml` is a statechart that runs for as long as the bridge is on, with the input `{ settings, cursor }`. It has `keep: 1d`, so its record holds every thing Bulldog may still take.

1. A job hears for up to 5 minutes, and emits `heard`, `{ things: [...], cursor }`, for what it hears.
2. When the job ends, the chart hears again, from the cursor the job returned. A bridge whose service cannot be asked to wait, such as GitLab, rests 30 seconds first.
3. When the job fails, the chart rests 30 seconds, then hears again.

```yaml
# ./.bulldog/bridges/telegram/in.yaml
statechart: telegram-in
keep: 1d
input:
  type: object
  required: [settings, cursor]
  properties:
    settings:
      type: object
      additionalProperties: false
      properties:
        places: { type: object }
    cursor: {}
initial: hearing
states:
  hearing:
    job:
      script: ./telegram.js
      input: { do: hear, settings: "${input.settings}", cursor: "${states.hearing.output?.cursor ?? input.cursor}" }
      allow: [net:api.telegram.org, "secret:TELEGRAM_TOKEN@net:api.telegram.org"]
      timeout: 5m
    on: { done: hearing, failed: resting }
  resting:
    timers: { again: 30s }
    on: { again: hearing }
```

Bulldog keeps one in run for each bridge with no problem, labelled `kind=bridge`, `bridge=<name>` and `made=<fingerprint>` of the bridge's definitions, the files they name, and its settings.

1. Bulldog keeps the bridge's newest run while its fingerprint is the bridge's, and cancels every other run of the bridge.
2. Once no run of the bridge is going, Bulldog starts a new one.
3. The new run is given the last cursor the old one emitted, or the cursor the old one was given when it emitted none. A new bridge's first in run is given no cursor.

A run that ends of itself is started again after a pause of 1 second, which doubles with each run that ends in a row, up to a minute.

The office's page shows each bridge as one of these:

| Status | When |
|---|---|
| `running` | an in run is going, and its last job did not fail |
| `failing` | a job or a run failed, until a job of the newest run ends well or hands something over; with Corbie's error |
| `stopped` | the bridge has a problem, or no in run is going |

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

An in hands over nothing its own bot or another bot sent. Bulldog cannot tell a bot from a person, so the in is the one place where this is done. An in hands over nothing that only some at the place may read, such as an internal note, since the answer goes where everyone reads it.

## What Bulldog does with a thing

Bulldog reads what each in run emits, in the order it was emitted, one thing at a time. It acts on each thing by the first of these that fits:

1. A press answers the wait whose `ref` it carries with the choice pressed, for the person whose identity pressed it.
2. Something a person says at `direct` answers a question carried to them, while it waits: the one it replies to, its `answers` the question's `sent`, or, replying to nothing, the latest carried to them on that bridge before it was heard.
3. Something said at a listed place, `direct` among them, starts a request of the place's team. Its asker is the person whose entry gives that identity on that bridge, or a guest where the place lets guests ask.
4. Anything else is let go.

- A text that is one of the question's choices, trimmed and in any case, answers with that choice.
- A decision takes only one of its choices. Anything else said to it goes on to 3.
- A press that gives no choice is let go.
- Requests answers a wait only when the person may answer it. An answer it refuses is let go.

For example, on Telegram:

1. Sam is asked "Is the boiler on?" in the message `4471:120`.
2. Sam replies to `4471:120` with "It is on.", which answers the question.
3. Sam is asked to decide "May the fixer order a part for £35?".
4. Sam says "Please order more oil.", replying to nothing. It is no choice of the decision, so it starts a request.
5. Sam says "Yes", replying to nothing, which answers the decision with `yes`.
6. Sam replies to `4471:120` with "Also, check the shed.". That question no longer waits, so it starts a request.

Each thing is taken once:

- A request heard is named `<client>/<stamp of at>-<keyed hash of the bridge and id>`. Bulldog looks for that name first, and starts only when there is no such run.
- A thing that answered a wait is known from the wait's answer, which holds the thing's id.
- A thing more than a day old when Bulldog reads it is let go.

A text longer than 32 KiB is cut at 32 KiB, with a line that says so, since a job's input is limited in size. A thing with no `id` or no `at`, or one Bulldog cannot read, is let go, and Bulldog reports why.

## Out

`out.yaml` carries one message. Its input is `{ settings, to, direct, text, choices, ref, answers }`, and it gives `{ sent }`, the id of the message it sent, or throws. For a text sent as several messages, `sent` is the last one's, so a reply to an earlier one starts a request.

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
- Each out run has the labels `kind=out` and `bridge=<name>`, and is named from what it carries: the request it answers, or the wait and the person it asks. Bulldog looks for that name first, and starts the run only when there is none. So a message is sent once.
- An out run that fails is shown on its request with Corbie's error, and is not tried again.
- A bridge with a problem carries nothing.

## The bridges Bulldog ships

| Bridge | Hears by | Sends to a person directly |
|---|---|---|
| `slack` | Socket Mode | yes |
| `telegram` | long poll | yes |
| `gitlab` | poll | no |
| `jira` | poll | no |

Each is a directory under `./.bulldog/bridges/`, with a `README.md` that says what the service needs, the settings, and what the bridge leaves out.
