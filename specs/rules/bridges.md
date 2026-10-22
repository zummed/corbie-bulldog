# Bridges

The rules behind the [Bridges overview](../bridges.md). Area code: `bridges`. A test stands in for the outside service: a shipped bridge's script is run against a server in the test's process, and Bulldog against an in and an out written for the test.

## A bridge in the office

**R-bridges-44** A bridge is a directory with `in.yaml` and `out.yaml`, named by `from:` as one Bulldog ships or the office's own, and its settings are its entry in `bridges:` but `from`.
*Accept:* the work office's `slack` reads the shipped Slack bridge, and its in and out are given `{ places }`. `from: ./bridges/pager` reads the office's own bridge. A `from` with no `out.yaml` is a problem of that bridge, and the rest of the office runs.

**R-bridges-45** `places` lists where requests are taken, each naming the team asked there, the office without one, and whether `guests` may ask; `direct` is any person's direct messages to the bot.
*Accept:* in the work office, a guest in #help asks `office`, Dana in #refunds asks `payments`, and a guest in #refunds starts nothing. With `direct` listed, Lee's direct message to the bot asks `office`.

## In

**R-bridges-46** `in.yaml` is a statechart with `keep: 12h`, given `{ settings, cursor }`, that runs for as long as the bridge is on: it emits `heard`, `{ things, cursor }`, for what it hears, hears again when its job ends, and rests 30 seconds when its job fails.
*Accept:* the shipped Telegram in, against a stand-in Telegram, emits a thing for each update with a cursor past it. When the stand-in answers 429 once, the in rests 30 seconds of the test clock, then hears again from the cursor it had.

**R-bridges-47** Bulldog keeps one in run for each bridge, labelled `made=<fingerprint>` of the bridge's definitions and settings. When the fingerprint changes or the run ends, Bulldog starts a new one, given the last cursor the old one emitted, and cancels the old.
*Accept:* adding a place to the Slack bridge's settings starts a new in run, given the old run's last cursor, and cancels the old run. An in run that fails is followed by a new one. A new bridge's first in run is given no cursor.

**R-bridges-48** A thing is `{ id, at, kind, from, place, where, text, files, mentioned, answers, ref, choice }`, each field as the overview's table gives it.
*Accept:* Kit's photo to the house on Telegram is handed over with `kind: said`, `from: "4472"`, `place: direct`, the caption as `text`, and the photo in `files` as a blob. A press of `yes` is handed over with `kind: press`, the message's `ref` and `choice: yes`.

**C-bridges-49** A shipped bridge gives as `from` the service's lasting id for a person, never a name or handle they can change. A changed handle would make a person a guest, or another person. *Test:* `bridges_from_is_lasting`

**C-bridges-50** A shipped in hands over nothing its own bot or another bot sent. A bot that hears its own answer at a place that takes guests answers it without end, and two offices' bots in one channel answer each other. *Test:* `bridges_bots_not_heard`

## What Bulldog does with a thing

**R-bridges-49** Bulldog acts on each thing in order: a press answers the wait whose `ref` it carries; something said at `direct` whose `answers` is the `sent` of a carried question answers that question, and one that replies to nothing answers the latest question carried to its sender on that bridge before it was heard, while that question waits; something said at a listed place starts a request of the place's team; anything else is let go.
*Accept:* Sam's press of `yes` answers the decision on the fixer's order. Sam's typed "yes", replying to nothing, answers the latest question carried to Sam. Kit's message in a group the settings do not list starts nothing.

**C-bridges-51** A person is known by the sender's identity, never by the place: a request's asker, a typed answer and a press are each matched on the thing's `from`, in a channel as in a direct message. Matching on the chat would let whoever shares it answer for another. *Test:* `bridges_identity_is_the_sender`

**C-bridges-52** A typed answer answers only a question carried to its sender before it was heard, and a thing that answered a wait is known from the wait's answer. After a restart Bulldog reads the things again, and an old answer must not answer a newer question. *Test:* `bridges_typed_answer_taken_once`

**R-bridges-50** A request heard is named `<client>/<stamp of at>-<keyed hash of the bridge and id>`. Bulldog looks for that name first and starts the request only when there is no such run, and lets go of a thing more than a day old when it reads it.
*Accept:* the same thing handed over by two in runs starts one request. A thing whose `at` is two days before the test clock starts nothing.

## Out

**R-bridges-51** `out.yaml` is given `{ settings, to, direct, text, choices, ref, answers }` and gives `{ sent }`, the id of the message it sent, or throws; an out whose input does not take `direct` sends to no person directly.
*Accept:* the shipped Slack out sends an answer to a thread, and with `direct: true` to Lee's direct messages, against a stand-in Slack. The GitLab and Jira outs take no `direct`, and a question to Al is never sent by GitLab.

**C-bridges-53** The ticket's value never leaves Bulldog: a message carries `ref`, a short keyed hash of the wait run, which names the wait and gives nothing. A message in an outside service is read by more than the one it is for. *Test:* `bridges_ticket_never_sent_out`

**C-bridges-54** A question, a decision and a sign-in link go to a person only in their direct messages, never to a channel, and a person no bridge can message directly is asked on the screens alone and sent no sign-in link. A channel is read by more people than those asked, and by more after a message is posted. *Test:* `bridges_only_direct_to_a_person`

**R-bridges-52** Each out run is labelled `kind=out` and `bridge=<name>` and named from what it carries; one that fails is shown on its request with Corbie's error, and is not tried again.
*Accept:* the answer to the guest in #help is carried by one out run, and carrying it a second time starts none. When the stand-in Slack refuses, the request shows REPLY as not sent, with the error, and no second out run starts.

## The bridges Bulldog ships

**R-bridges-53** Bulldog ships `slack`, which hears by Socket Mode, `telegram`, by long poll, and `gitlab` and `jira`, by poll. The Slack and Telegram outs take `direct`, the GitLab and Jira outs do not, and each bridge has a `README.md`.
*Accept:* each shipped bridge, against its stand-in service, hands over a message said at a listed place and sends an answer to its `where`. Each shipped bridge directory holds a `README.md`.

## Interfaces

Uses:

- **Office.** The bridges with their settings, each person's identities, and the office's secret, which keys `ref` and the names of requests heard.
- **Requests.** Hands it each thing: a new request, or an answer to a wait.
- **Corbie.** Its client, to start, follow and cancel in runs and to start out runs.

Used by:

- **Requests.** Carries a message to a place, or to a person directly, through an interface the program fills.
- **Screens.** Sends a sign-in link to a person directly.
