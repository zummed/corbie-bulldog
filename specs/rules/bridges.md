# Bridges: rules

The detailed rules behind the [Bridges overview](../bridges.md). Requirements are R-bridges-nn, constraints C-bridges-nn. A "stand-in bridge" is a bridge folder written for the tests, whose machines answer as they are told to; a "stand-in service" is a local server built for the tests that speaks as an outside service does. Neither needs the network.

## The folder

**R-bridges-01** A bridge is a folder holding an in machine, `in.yaml`, and an out machine, `out.yaml`, with the scripts they use, and each is sent to Corbie exactly as it is written, but for a model named by one word, which is the office's model of that name, as for a worker of the top team (Office).
*Accept:* an office lists a stand-in bridge. What Bulldog hands Corbie's client to start its in run and its out run is the two files and their scripts as they are in the folder, byte for byte, and nothing else. With the office's `models` naming `usual`, and the stand-in out machine an agent written with `model: usual`, its out run is handed that file with the office's `usual` model in place of the word, and nothing else changed.

**R-bridges-02** The top team's file lists the office's bridges under `bridges`, each by its name and with settings of its own; a bridge's machines are those of the folder Bulldog ships that its `from` names, which is only one under `.bulldog/bridges/`, or else those in `bridges/<name>`, and its settings are handed to both of them, whole and as written, `from` among them, as the input `settings`; of them Bulldog reads only `from` and `places` (R-bridges-39).
*Accept:* the top team's file lists `slack: { places: { "#help": { guests: true } }, anything: 3 }`, the last a setting Bulldog has no use for. The in run and the out run Bulldog starts are of `bridges/slack/in.yaml` and `bridges/slack/out.yaml`, and each is given as the input `settings` the mapping `{ places: { "#help": { guests: true } }, anything: 3 }`, unchanged. Listed as `slack: { from: ./.bulldog/bridges/slack, places: { "#help": { guests: true } } }`, its runs are of `.bulldog/bridges/slack/in.yaml` and `.bulldog/bridges/slack/out.yaml`, as Bulldog ships them, and are given those settings, `from` among them.

**R-bridges-03** A bridge's name is the key a person's identity on it is written under, so `slack: U0DANA00` is the person the bridge named `slack` gives as `U0DANA00`.
*Accept:* an office names a bridge `slack` and Dana with `slack: U0DANA00`. A thing heard on that bridge from `U0DANA00` is Dana's; the same identity heard on a bridge named `work-slack` is a guest's.

**R-bridges-04** An in machine that takes `cursor` listens, and one that takes `delivery` hears. A broken bridge stops only itself, as a broken worker does: an entry under `bridges` with no `from` and no folder at `bridges/<name>`, a `from` that names no folder Bulldog ships under `.bulldog/bridges/`, an entry with `from` and a folder at `bridges/<name>` too, a folder there with no entry, a folder missing either machine, an in machine that takes both inputs or neither, a machine Corbie refuses, settings that are not a mapping, and settings whose `places` are not right (R-bridges-39), a team that is not there among them, are each a problem of that bridge, shown on the office with every other problem found; that bridge is not run until it is mended, and the rest of the office reads and runs.
*Accept:* the work office with its GitLab in machine taking both inputs, `jira` listed under `bridges` with no folder, a folder `bridges/teams/` that `bridges` does not name, and `matrix` listed with a folder that has no `out.yaml`. Reading the office reports the four problems at once, each naming its bridge, and the office reads; no run of any of the four is started; the Slack bridge hears, and what Dana asks in #help goes to the bot.

**C-bridges-01** No bridge is called `chat`. That is the built-in chat's name, under which a person is never written an identity. *Test:* `bridges_chat_name_reserved`

**C-bridges-02** Of a bridge's settings Bulldog reads only `from` and `places`, and of each place only its `team` and `guests`; everything else is handed on and never read. How a bridge reaches its service is the bridge's business; reading it in Bulldog would make every new service a change to Bulldog. *Test:* `bridges_settings_passed_unread`

## Places

**R-bridges-39** A bridge's settings list under `places` the places it listens in, each by its name in the bridge's own words, a channel or a project, or `direct`, which is anything said to the bridge directly; each place may give `team`, the name of the team asked there, the office if it gives none, and `guests`, true or false, false if it gives none. Anything else in a place is the bridge's. Settings with no `places` listen in no place. A `places` that is not a mapping, a place that is not a mapping, a `team` that is no team of the office, which Office checks, and a `guests` that is not true or false are each a problem of that bridge (R-bridges-04).
*Accept:* the work office's Slack settings `places: { "#help": { guests: true } }` give #help to the office, with guests; its GitLab settings `places: { platform/api: { team: platform } }` give platform/api to platform, with no guests. `places: { "#help": { team: billing } }`, the work office having no team `billing`, `places: { "#help": { guests: maybe } }` and `places: ["#help"]` are each a problem naming the Slack bridge; the Slack bridge is not run, and the GitLab bridge listens. The house's `telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }` gives what is said to the bot directly to the house, with no guests; with `telegram: { from: ./.bulldog/bridges/telegram }` in its place, the house reads, and its Telegram bridge listens in no place.

**R-bridges-40** A thing heard in no listed place, its `heard_in` missing or naming a place the settings do not list, is taken only when it answers a thing the out machine sent its sender directly, a choice among them; anything else from there begins no request, is an entry in none, and has nothing sent back.
*Accept:* the work office lists only #help on Slack. Dana writes in #random, and then to the Slack bridge directly: no request is made, no entry is written and no out run is started. A decision carried to Lee directly is answered by his press of yes, and a question carried to Priya directly by her typed reply: both are taken. With `direct: {}` added to the Slack places, Dana's direct message begins a request to the office, its origin her direct place.

**C-bridges-33** Which team is asked at a place, and whether guests may ask there, is read from the settings, never from the in machine, which says only which listed place a thing was heard in. Who may ask the office, and of which team, is then written where a change to the office shows it and a person decides it, not inside a bridge's script. *Test:* `bridges_places_read_from_settings`

## In: listening

**R-bridges-05** For each bridge that listens, Bulldog keeps exactly one listening run going: it starts the in machine with the settings and the cursor it holds, and when that run ends well it takes what was heard, keeps the cursor the run gave, and starts the next.
*Accept:* a stand-in in machine ends at once each time with one thing and a cursor one higher. After five runs Bulldog has taken five things, holds cursor 5, and at no moment were two listening runs of the bridge alive.

**R-bridges-06** The first listening run of a bridge is given no cursor, and the in machine then hears only what comes after it.
*Accept:* a stand-in service holds three old messages. The shipped Telegram in machine's first run, given no cursor, ends with nothing heard and a cursor past them; a message sent after it is heard by the next run.

**R-bridges-07** A listening run that fails is started again after a pause that grows with each failure in a row, up to a minute, and the failure is shown to the office's people on the screens until a run ends well.
*Accept:* a stand-in in machine fails three times, then ends well. The runs start further apart each time, never more than a minute apart; the screens show the bridge's failure with Corbie's error after the first, and nothing once the fourth has ended well.

**C-bridges-03** A cursor is kept only after the things heard with it have been taken. Kept first, a stop between the two loses what was heard; taken first, a stop between the two hears it again, and R-bridges-11 takes it once. *Test:* `bridges_cursor_kept_after_taking`

## In: hearing

**R-bridges-08** A bridge that hears has an address on the server, `/bridges/<name>`; what arrives there is handed unread, as the delivery, to a new in run, and the address takes no sign-in, since the in machine checks with the secret it holds that the delivery is the service's. Being called there is the service speaking, not Bulldog.
*Accept:* a POST to `/bridges/slack` with a body and two headers starts one in run whose `delivery` holds the method, the headers and the body exactly as sent. A stand-in in machine that finds the signature wrong ends with nothing heard, and nothing is taken.

**R-bridges-09** The answer to the caller is what the in run says to answer, if the run ends within two seconds, and otherwise that the delivery was received; a run that ends later is still taken.
*Accept:* a stand-in in machine ends at once with an answer body `challenge-1`; the caller receives `challenge-1`. Another takes five seconds; the caller is answered as received after two, and the thing it heard is taken when it ends.

**C-bridges-04** A delivery larger than the server takes is refused with a status that says so, and is never passed on cut short. An in machine given half a body reads it as something else, or as nothing. *Test:* `bridges_oversized_delivery_refused`

**C-bridges-05** A delivery's body and headers are never written to the server's log. They carry a service's signature, and sometimes its keys. *Test:* `bridges_delivery_not_logged`

**C-bridges-06** A delivery to a bridge that is not listed, or that listens, is refused, and no run is started. The address is open to anyone, and only a listed bridge that hears has a machine to judge what came. *Test:* `bridges_delivery_only_to_hearing_bridge`

## What is heard

**R-bridges-10** An in run ends with `heard`, a list of things, each with: `id`, the service's own id for it; `kind`, which is `said`, `choice` or `status`; `from`, the sender's identity as the service vouches for it; `heard_in`, the listed place it was heard in, by its name in the settings, if any; `place`, where it was said, as narrow as the service allows; `threadless`, true when that place holds no threads, as a chat does, false if it is not given; `readers`, the identities that can read the place, where the service can tell; `about`, the source thing it is about, with a link; `answers`, the reference of the sent thing it answers, or the `id` of the thing heard it answers, if any, which for a choice is the `ref` it was sent with; and for its kind, the `text` and `files` said, the `choice` made, or the `status` now in the source's words and whether the thing is now `closed`.
*Accept:* a stand-in in machine ends with one thing of each kind, every field filled. Each is taken with every field as given, and a thing with no `id`, no `kind` or no `from` is refused, the bridge's failure shown on the screens naming the field.

**R-bridges-11** Each thing heard is taken once, by its bridge and its `id`, whichever run or delivery gave it.
*Accept:* two listening runs both end with the thing of id `m7`, and a delivery gives it a third time. One entry is made of it.

**R-bridges-12** A thing said becomes an entry in the request whose sent thing it answers, said at the place that thing was sent to, when its sender may still answer it (C-bridges-29); failing that, heard in a listed place and answering a thing heard earlier at its place, in that thing's request, when its sender may say more in it. Failing those, at a place that holds threads, it is an entry in the latest request whose origin is its place, open or answered, when its sender may say more in it, as Requests says who may, and an answered request is then working again; at a threadless place it is the answer to the latest question carried there, when that still waits on its sender (C-bridges-35). Failing all of these, heard in a listed place (R-bridges-39), it begins a new request to the team that place names, the office if it names none, whose origin is its place and whose asker is its sender, and that team's lead starts on it.
*Accept:* a thing from Dana that answers a question the out machine sent Dana for request 4 becomes an entry in request 4. A thing said in the thread that is request 5's origin becomes an entry in request 5; another said there after request 5 is answered is an entry in request 5 too, and request 5 is working again. A thing said in a new thread of #help begins request 6, to the office, with that thread as its origin and its sender as its asker, and the bot starts on it. Raj opens an issue in platform/api: it begins request 7, to platform, with the issue as its origin, and ops starts on it. In the house, Sam writes "the socket sparked" to the Telegram bot, a threadless place, and it begins request 1; the steward's answer is sent as two messages. "The bins", replying to nothing, begins request 2. A reply to the first of the two messages, and a reply to his own "the socket sparked", are each an entry in request 1. The steward then puts a question to Sam for request 1: the next thing he says, replying to nothing, answers it, and the one after that begins request 3.

**R-bridges-13** Where a thing is said is as narrow as the service allows: a thread rather than its channel, an issue or a merge request rather than its project; the listed place it is heard in is the wider one the settings name.
*Accept:* the shipped Slack in machine, given a message at the top of #help and a reply under it, gives both the same place, the message's thread, and not #help, and gives both as heard in `#help`.

**R-bridges-14** Files said on a bridge are copied out of the in run into Bulldog's store before the run's record can go, and the entry holds them.
*Accept:* a stand-in in machine ends with a thing holding a picture. Once the run's record is removed, the entry still shows the picture, whole.

**R-bridges-15** A choice is given to the decision whose reference it carries, with the checked identity of who made it.
*Accept:* the stand-in out machine sends a decision with reference `d12`; the in machine then ends with a `choice` of yes on `d12` from Lee's identity. The decision is given yes from Lee.

**R-bridges-16** The readers a thing gives for its place are matched to people and replace the readers of the origin it belongs to; a thing that gives no readers leaves them as they were; and an origin whose place has never given any is read by its asker alone.
*Accept:* a thing in #help gives the identities of Dana, Al and a guest; the origin's readers are Dana and Al. A later thing there gives no readers; they are still Dana and Al. A later one gives Dana alone; the readers are then Dana alone. A thing from a direct place gives none; its origin is read by its sender alone.

**C-bridges-30** An in run, listening or hearing, that was going when the server stopped is found again by its labels when it is back. What it heard is taken, and the cursor a listening run ended with is kept, as after any run; a listening run still alive is waited on, never joined by a second. A delivery answered as received has no copy but its run, so it would be lost; a cursor not kept hears again what was taken; and two listening runs of one bridge hear everything twice. *Test:* `bridges_in_run_read_after_restart`

## Out

**R-bridges-17** For each thing to carry on a bridge, Bulldog starts the out machine once, with `settings`; `to`, the place to say it in; `direct`, whether that is a person's direct place; `text`, a blob; `files`, each `{ name, data }` with `data` a blob; `answers`, the reference of the thing heard that it answers, if any; and for a question or a decision, `choices` and `ref`, a reference of Bulldog's that the in machine hands back with the choice.
*Accept:* the bot answers Dana's request. One out run of the Slack bridge is started, given the settings, Dana's thread as `to`, the answer's text and the reference of Dana's message as `answers`.

**R-bridges-18** An out run ends with `sent`, the service's reference to what it sent, or a list of references, one for each message it sent; or with `refused`, the service's refusal in words a person can read. Bulldog keeps every reference, so a later thing heard that answers any of them is matched to what it answers.
*Accept:* the stand-in out machine ends with `sent: s9`. A thing heard later with `answers: s9` is an entry in the same request. Another out run ends with `refused: "the channel is archived"`, and the request shows those words. A third ends with `sent: [s10, s11]`; a thing heard with `answers: s10` is an entry in the request too.

**R-bridges-19** An answer, and anything said to the asker, goes to the request's origin; anything else for a person goes to their direct place on each bridge that knows them, and on the screens. A sub-request has no place, only the request that asked it: its answer, and what is said to its asker, go to the run that asked, and no bridge carries them.
*Accept:* in the work office, a question from the bot to Dana is carried to her thread in #help. A decision for Lee and Priya is carried to each of them directly on Slack and is on the screens of both, and nothing of it goes to #help. The ledger's answer to payments' sub-request starts no out run of any bridge.

**R-bridges-20** A bridge whose service has no direct place carries only what goes to an origin, and a person known only on such bridges is reached on the screens. A bridge's out machine says that its service has a direct place by taking the input `direct`.
*Accept:* Lee is written with a GitLab identity and no other. A decision for Lee starts no out run of the GitLab bridge, and is on Lee's screens.

**C-bridges-07** A bridge is given only what Requests says the people at its place may see, judged when the out run is started, not when the thing was made. Someone taken out of a team since must not be sent what it then holds. *Test:* `bridges_carried_only_if_seen_at_sending`

**C-bridges-08** Nothing is echoed back to whoever said it: what a person said is never carried to them. A person's own words sent back to them read as someone else's. *Test:* `bridges_nothing_echoed`

**C-bridges-09** A question or a decision is carried once, as itself, and the entry that leads to it is not carried as well. Otherwise the person reads it twice, once as talk and once as the thing to answer. *Test:* `bridges_question_carried_once`

**C-bridges-10** Only entries addressed to a person, or answering the origin, are carried. What workers say to each other, and what a team read, stay inside the team. *Test:* `bridges_inner_talk_never_carried`

**C-bridges-11** Where a thing goes is the origin the request was made with, or the person's own identity as written in the office, and is never worked out afterwards from what else was said. A place guessed later can be the wrong one, or a wider one. *Test:* `bridges_place_fixed_with_request`

**R-bridges-38** A one-time sign-in link is carried when the server asks, by one out run, to the person's direct place on the bridge the server names. It is no entry of any request and is shown on none; what it carries is kept only until its run is given it; the bridge does not try it again; and how its run ended, `sent`, `refused` or failed, is given back to the server, which says whether another of the person's bridges is tried.
*Accept:* the house, with a stand-in bridge, as this page means one, listed as `telegram` in place of the shipped one; the server asks for a link to Sam on it. One out run is started, with Sam's direct place as `to` and a text holding a link under `/signin/`, and no request gains an entry. The stand-in out machine refuses it: no second run of it is started, and the server is told it was refused, with the refusal's words.

**C-bridges-31** A one-time sign-in link is carried only to a person's direct place, never to an origin, and a bridge with no direct place carries none. A link in a shared place signs in whoever opens it first. *Test:* `bridges_sign_in_link_only_direct`

**C-bridges-12** The `ref` handed to the out machine is Bulldog's own and grants nothing; a Corbie ticket's value never leaves Bulldog. A ticket is the right to answer, and a message in an outside service is read by more than the one it is for. *Test:* `bridges_ticket_never_sent_out`

## Who is speaking

**R-bridges-21** A thing heard is matched to the one person written in the office with its `from` on that bridge, exactly as written; a thing matched to nobody is a guest's.
*Accept:* Dana is written with `slack: U0DANA00`. A thing from `U0DANA00` is Dana's; one from `u0dana00` and one from `U0GUEST0`, which no person in the office has, are guests'.

**R-bridges-22** A guest may ask only in a listed place whose settings give `guests: true`, `direct` among them, and is answered where they asked. What a guest says anywhere else is not taken, and nothing is sent back; a choice a guest makes anywhere else is given to no decision, writes nothing, and is answered, as every choice is.
*Accept:* with the work office's `"#help": { guests: true }`, someone the office does not name asks in #help: a request is made to the office with them as a guest asker, its origin their thread, and the answer is carried there. With `guests` taken out of #help, the same message makes no request and starts no out run. The house's Telegram bridge with `places: { direct: {} }`: an unknown identity writes to it directly, and no request is made and no out run is started; with `direct: { guests: true }`, the same message makes a request to the house with them as a guest asker, its origin their direct chat, and the answer is carried there.

**R-bridges-23** A request a guest asks shows the bridge and the identity the bridge gave, so that a person can be written down for them.
*Accept:* an unknown identity `9001` asks directly on the house's Telegram bridge, whose places give `direct: { guests: true }`. The office's people see the request's asker as a guest, on `telegram`, as `9001`.

**C-bridges-32** A guest's thing is taken only from a listed place whose settings give `guests: true`, whatever the in machine says, and a place that says nothing of guests takes none. Anyone on a service can write in a channel or to a bridge directly, and an office that did not choose to hear strangers there must not be asked by them, however its bridge is written. *Test:* `bridges_guests_off_unless_said`

**C-bridges-13** Identity is the sender's, never the place's: a message and a choice are matched on the same identity, in a group as in a direct place. Matching one on the chat and the other on the user lets a group's messages in and never takes a press made there. *Test:* `bridges_identity_is_the_sender`

**C-bridges-14** Every choice heard is answered with one sentence, carried back to whoever made it at the place they chose, whether it is taken or not; what the sentence says is the Decisions area's. A press that nothing answers leaves the person's client waiting, and them not knowing why. *Test:* `bridges_every_choice_answered`

**C-bridges-15** Identity is taken only from what an in run ended with, never from what was delivered to the server. Only the in machine holds the secret that tells the service's word from anyone's. *Test:* `bridges_identity_only_from_in_run`

**C-bridges-16** Where anything is sent back to someone over a bridge, when something they said is not taken, it is one sentence written for a person, never an error's own text. An error's text holds names and details from inside the office. *Test:* `bridges_refusal_is_a_sentence`

**C-bridges-29** Whether a thing said is an entry in a request is judged as the office is read now, by Requests' rule of who may. A thing said in a direct place that answers something sent there is taken only from the person it was sent to, and only while it would still be sent to them; otherwise it is answered with one sentence, and nothing is written. Any other thing that answers a sent thing or a thing heard, or is said at a request's origin, is an entry in that request only when its sender may say more in it. Without this, someone taken out of a team answers into its work by replying to an old message, and a reply in another's direct chat speaks for them. *Test:* `bridges_reply_checked_against_sight_now`

**C-bridges-34** A person's identity on a bridge is changed only by a change to the office's files, decided like any other, their own entry included; nothing a bridge hears and nothing done on the screens outside a change writes one. A yes is taken from that identity, so whoever changes it takes the person's yes, which is what someone taking over their account would do. *Test:* `bridges_identity_changed_only_by_a_change`

**C-bridges-35** At a threadless place, a thing said that answers no thing sent or heard, and no question that waits on its sender there, begins a new request, and is never an entry in an earlier one; words with a decision waiting there begin one too, since a decision is answered by a press or a reply. A chat with a bot holds every conversation its person has with it, and taking the latest request there as the one meant would run every new ask into the first. *Test:* `bridges_threadless_place_begins_new_request`

## Delivered once

**R-bridges-25** A thing is delivered when its out run ends with `sent`, and only then.
*Accept:* a stand-in out machine that takes ten seconds: the thing shows as being sent until the run ends with `sent`, and as delivered after.

**R-bridges-26** A thing whose out run ends with `refused` is given up at once; one whose out run fails is tried again with a new run, each later than the last, and given up after five runs; a thing given up is shown on its request as not delivered, with why.
*Accept:* a stand-in out machine refuses one thing and fails every time for another. The first is shown not delivered after one run, with the refusal's words; the second after five runs, further apart each time, with Corbie's error.

**R-bridges-27** Things to one place are carried in the order they were made, one at a time, and a thing given up does not hold up the things after it.
*Accept:* three answers go to Dana's thread; the stand-in out machine fails every time for the second. The first is sent, the second is given up after five runs, and the third is sent after it, never before the first.

**R-bridges-28** An out run started before the server stopped is found again by its labels when it is back, and how it ended is read from it; a thing is not carried again while a run of it is alive or has ended with `sent`. How a run ended is taken once: handed back again, as when the server stopped after it was taken and before work kept that it was, it changes nothing and is not refused.
*Accept:* the server is stopped while an out run is under way and started after the run has ended with `sent`. The thing is delivered and no second out run of it is started. One run's `sent` and another's failure are each handed back twice: the first thing is delivered once, the second waits to be carried again after one run, and neither hand-back is refused.

**R-bridges-29** A bridge newly listed, a bridge newly mended, or a person newly given an identity on one by a change, starts from now: nothing said before is carried to them, and every question and decision that still waits for that person is. What was to go on a bridge while it was stopped is given up, and shown so.
*Accept:* Lee is given a Slack identity, by a change decided like any other, while a decision waits for him and after three answers were given in the office. One out run carries the decision to Lee, and none carries the answers.

**C-bridges-17** One bad thing never blocks a place, a person's direct place included. A thing the service refuses for good would otherwise be tried for ever, and nothing after it would arrive. *Test:* `bridges_bad_thing_blocks_nothing`

**C-bridges-18** Places are carried to apart, and listening goes on while carrying waits. A slow or unreachable service holds up only its own place, not every person or what comes in. *Test:* `bridges_slow_place_holds_no_other`

**C-bridges-19** A thing whose out run sent it but then failed, before ending with `sent`, may be sent twice; this is the one way a thing is sent twice, and it is chosen over losing a thing. A person reads a repeat as a repeat; a lost answer they never learn of. *Test:* none, since the window is inside the out machine's own run, where the bridge, not Bulldog, decides whether to try a send again.

**C-bridges-20** A thing being taken is in its request before anything is carried because of it, so what is carried can be checked against where it came from. Without that order a person's own words can be carried back to them before they are known to be theirs. *Test:* `bridges_taken_before_carried`

## Keys and errors

**R-bridges-30** A bridge's keys are Corbie secrets named in its definitions and bound to the service's host, but for a secret used only as a key to sign or check with, such as Slack's signing secret, which Corbie lets be used so only when it is bound to anywhere; Bulldog never reads, holds or is given one, and a bridge needs nothing in the server's environment.
*Accept:* the shipped Telegram bridge runs with its token set as a Corbie secret and nothing set for the server; the server's environment, its store and its log hold no trace of the token.

**R-bridges-31** A failed bridge run is shown and logged with the error Corbie gives, which Corbie has masked, and nothing of a key appears where a person reads it.
*Accept:* a stand-in service fails every call with an error that repeats the address it was called at, the token inside it. The bridge's failure on the screens and in the log names the token's secret and never its value.

**C-bridges-21** A bridge's script never builds an error of its own from a whole address that holds a key. Corbie masks what crosses from plugin to script, but not what a script makes up. *Test:* `bridges_shipped_errors_hold_no_key`

## Status at the source

**R-bridges-32** A `status` heard for a source thing is handed to Requests for the request whose origin it is, with whether the thing is `closed`, who changed it, matched as any sender is, when it was heard and a link to the thing, in place of the status heard before; how it is shown, and what a close does, are Requests'.
*Accept:* a GitLab issue that began request 7 is closed in GitLab by Raj. The in machine hears `status: closed`, `closed: true`, from Raj's GitLab identity; Requests is given that status for request 7, closed, by Raj, with the time and the issue's link, and holds no earlier status beside it.

**R-bridges-33** A `status` heard for a thing that is no request's origin makes nothing: no request, no entry and no run.
*Accept:* the stand-in GitLab in machine hears `status: closed` for issue 99, which began no request. No request is made and no run is started.

**C-bridges-22** Bulldog never sets a status at the source, and holds of it nothing but the last status heard: no out run is started to change one. Two keepers of a status disagree, and the source's is the one its people read. *Test:* `bridges_no_status_copy`

## The built-in chat

**R-bridges-34** The built-in chat is a bridge with no folder and no runs: its sender is the signed-in person, its place is the chat, read by the asker alone, and what is for a person reaches them on their screens and is delivered when it is there.
*Accept:* Kit writes in the chat. A request is made with Kit as asker and the chat as origin, no run of any bridge is started for it, and the steward's answer is on Kit's screens and shown as delivered.

## The bridges Bulldog ships

**R-bridges-35** Bulldog ships four bridges as folders in plain Corbie, each in `.bulldog/bridges/<name>/` with its page and the entry it is set with, `settings.yaml`: Telegram and GitLab, which listen; Slack, which hears; and Jira, which listens.
*Accept:* an office whose `bridges` lists each shipped bridge with the entry it is set with reads with no problem; each runs its folder in `.bulldog/bridges/`, checked by Corbie, and each in machine takes `cursor` or `delivery` as stated here.

**R-bridges-36** A shipped bridge is used in place: a change writes its entry under `bridges` with its `from`, and nothing of its folder is copied into the office, so it runs as the Bulldog that serves the office ships it. The shipped Telegram bridge's entry is `from: ./.bulldog/bridges/telegram` and `places: { direct: {} }`, which hear people's direct messages to the bot. A bridge copied into `bridges/<name>`, its `from` taken out, is the office's own, read and changed like any other file of it, and a later Bulldog's shipped files do not change it. A new office has only the built-in chat.
*Accept:* in a new house, which writes no `change` list and whose `everything` names Sam, Sam's change sets `telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }` under `bridges` and touches no other file; the check says the bridge is new and sets it apart. Once it is decided, the office's history holds, after the office's first commit, one change by `sam`, of `office.yaml` alone, and the Telegram bridge runs `.bulldog/bridges/telegram`.

**R-bridges-37** Telegram and Slack carry questions and decisions to a person directly, with a button for each choice, and hear a press as a choice; GitLab and Jira answer in the issue or merge request they were asked in, and hear a change of its state as a status.
*Accept:* against a stand-in Telegram, a decision for Sam is sent to Sam's chat with a yes and a no button, and Sam's press of yes is heard as a choice of yes from Sam's identity. Against a stand-in GitLab, an answer is a comment on the issue, and closing the issue is heard as `status: closed`.

**C-bridges-24** A shipped bridge gives as `from` the service's lasting id for a person, never a name or handle they can change. A changed handle would make a person a guest, or another person. *Test:* `bridges_shipped_identity_is_lasting`

**C-bridges-25** A shipped out machine splits text the service would refuse as too long into messages in order, counting as the service counts, and never cuts it. A cut answer loses its end, and one the service refuses fails every time. *Test:* `bridges_shipped_long_text_split`

**C-bridges-26** A shipped bridge tells the service's errors apart by their codes, never by searching their words. A service's words change; its codes are what it promises. *Test:* `bridges_shipped_errors_by_code`

**C-bridges-27** A shipped out machine that answers a message the service no longer has sends its text all the same, not in reply. A person may delete what a thread began with, and what follows should still arrive. *Test:* `bridges_shipped_reply_to_gone_message_still_sent`

**C-bridges-28** The shipped Slack bridge's in machine refuses a delivery whose signature is wrong or more than five minutes old. Its address is open to anyone, and an old delivery sent again is a replay. *Test:* `bridges_slack_delivery_signed_and_fresh`

**C-bridges-36** The shipped Telegram bridge gives every thing said as `threadless`, and its out machine ends with `sent` naming every message it sent, the parts of a long text and each file among them. A chat, and a topic of a group, holds no threads on Telegram, only replies, and a person replies to whichever part of an answer they were reading. *Test:* `bridges_telegram_threadless_every_message_kept`

**C-bridges-37** The shipped GitLab and Jira bridges reach the host their settings' `site` names, and no other: their machines allow `net:${input.settings.site}` and send their token to that host alone, and their scripts name no host of their own. A bridge used in place is never edited by the office, so a GitLab or a Jira of its own is reached only by a setting; and the token goes only where the office's file, which a person writes, says. With no `site`, Corbie cannot fill the permission, and the run fails before anything is sent. *Test:* `bridges_shipped_site_from_settings`

**C-bridges-38** A shipped bridge's in machine reads on from any cursor an earlier version of it gave, hearing only what came after it, or its page says under Changed what is lost. The cursor is kept across an upgrade of Bulldog, and a shipped bridge used in place changes with Bulldog; a cursor it could not read would hear everything again, or nothing. *Test:* `bridges_shipped_reads_an_earlier_cursor`

## Interfaces

- **Office** reads the `bridges` listing, the folder Bulldog ships that each `from` names (R-bridges-02), and each person's identity on each bridge, checks that each team a place names is the office's (R-bridges-39), says which model a word names in a bridge's machines (R-bridges-01), checks both machines of every bridge with Corbie, and shows a broken bridge as a problem of that bridge, which stops only it while the rest of the office reads and runs (R-bridges-04, C-bridges-01, R-office-21).
- **Requests** takes each thing said as an entry, or as a new request to the team its listed place names, with its origin, whose lead starts on it (R-bridges-12), takes a guest's asking only from a place that takes guests (R-bridges-22), and makes an answered request working again when more is said in it (R-bridges-12), says who may say more in a request and who may still answer what was sent (C-bridges-29), keeps that a sub-request has no place (R-bridges-19), keeps an origin's readers (R-bridges-16) and its source's last status, and stops a request's work when its source thing is closed (R-bridges-32), keeps the files copied out of an in run (R-bridges-14), and says what the people at a place may see, which is all a bridge carries (C-bridges-07).
- **Decisions** is given each choice with the checked identity of who made it (R-bridges-15), and says what the one sentence answering it holds (C-bridges-14).
- **Work** starts every in and out run with its labels, from the definitions as Office filled them (R-bridges-01), and finds them again by those labels after a restart (R-bridges-28, C-bridges-30).
- **Server** serves the built-in chat (R-bridges-34) and the address at which a bridge that hears is called (R-bridges-08); it has a bridge carry a one-time sign-in link to a person's direct place, choosing which of their bridges, and is told how the run ended (R-bridges-38, C-bridges-31).
- **Changes** writes a bridge's entry, a shipped bridge's `from` among it (R-bridges-36), any change to a bridge or its places, and any change to a person's identity on a bridge, their own included, as a change decided like any other (R-bridges-36, C-bridges-34).
