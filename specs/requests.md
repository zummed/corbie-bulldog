# Requests

A request is one thing someone asked. It is asked on a bridge, in the built-in chat or by a routine, and it goes to one team: a request from a bridge goes to the team the place it was said in names, in the bridge's settings, or to the office where the place names none; one from the chat to the team the person picks, or the office; one from a routine to its team. This area keeps every request and everything said in it, and decides who may see which part of it and who may say more in it. It knows the office only as the files say it is, read each time, and Corbie's runs only by their labels.

## A request

A request has a number, given once in the office and never again; the team it went to; its asker; its origin; and a thread of entries. The team's lead starts on it, or, for a routine, the worker the routine names. A team whose lead is a person waits for that person, and their answer is the request's answer.

The **asker** is whoever asked: a person, a guest, or, for a routine, the team itself. A person or a guest is kept as the identity their bridge checked, or, for the chat, as the person signed in, and who that is is read from the office each time it matters. A name written again for someone new inherits nothing.

The team is told who is asking, read from the office when its run is given the request: a person, by their name and the teams they are in; or a guest, as a guest, with the bridge and the identity it vouched for. Whether a guest may have what they ask is the team's own to judge, by its workers and its decisions, as it judges anything else; Bulldog adds no permission of its own for it.

The **origin** is the place a request was asked: a Slack channel or thread, a GitLab issue, the built-in chat. It fixes who reads the answer, and, on a bridge, which team is asked: each place a bridge's settings list names its team, or none for the office, and a direct message to the bridge is a place like any other, heard only when it is listed. Every answer goes back there, and whoever can read the place may see what was done and say more in it. Who can read a place a bridge listens to comes from the source: it is the people whose identities the bridge's in machine last reported as reading it, and a place it never reported is read by the asker alone. The chat is read by the asker alone. A routine's request has its routine as its origin, read by the people of its team, and, when one of them ran it now on the screens, keeps who did. A sub-request is asked at no place and has no origin of its own: it has the request that asked it.

## The thread

Everything said in a request is an entry: who it is by, who it is to, when, its words, and any files with it. The asking is the first entry. After it come replies, what workers asked people and told them, the sub-requests made, the decisions with who decided and when, and the answers. An entry is written once and never changed. Times are kept as instants and shown in the office's zone, the `zone` of the top team file.

Bulldog writes a few entries itself: that a run failed (to the team with the error, which its people are told as they are when it is shown or sent, and to the asker without it), that a request was stopped and by whom, or in place of who a line of its own, such as that a team did not disclose its answer, that its issue was closed at the source and by whom, that a team or a person has gone from the office, which answers what was open to them. They are by Bulldog, and no worker or person can be taken for it.

## States

A request that began in Bulldog, in the chat, by a routine, as a sub-request, or at a place that keeps no status of its own, is in one of three states.

- **Working**: its team is on it, and nothing in it waits on a person.
- **Waiting on someone**: a question put to a person, a decision that has reached people, or a lead who is a person, waits for them. The state names who.
- **Answered**: its lead's answer has been said, and nothing has been said in it since. An empty answer to a routine, or at a channel place, any place on a bridge but its direct place, says nothing: its request is answered with no entry, and nothing goes to the team's people or to the place. This holds for the first thing said there as for any after it.

Something said in a request by someone who may say more in it is an entry in it, whether the request is open or answered, and makes an answered request working again. An answer held by its team's disclosure check is not yet said: until it is let go the request is working, or waiting on the people the disclosure has reached.

A run that ends with no answer, because it failed or was stopped, while nothing else in its request is open, leaves the request answered. Bulldog's entry is its last, and goes to the origin, or for a sub-request to the run that asked: for a failure, it says only that the team could not answer, and the team's people are told the error; for a stop, it says who stopped it. Saying more makes it working again.

A request from something that keeps a status of its own, such as a GitLab issue or a Jira issue, shows that status as its bridge's in machine last reported it, with when, and has none of these three states. Beside it is shown whether its team's work on it is going, or waits on someone. Bulldog never sets that status. A change of it is not an entry and starts nothing, with one exception: the issue being closed at its source stops the request's work, as a stop does, with Bulldog's entry saying who closed it.

## Sub-requests

Work that needs another team becomes a sub-request: a request of its own to that team, with a run of its own, made by a tool Bulldog ships. It is asked at no place: it has the request that asked it, and its answer goes back to the run that asked. The team asked is told the origin at the top of the tree, which is who will in the end read what it says.

Who may say more in a sub-request is the team that asked it, its people and, through their runs, its workers, and the people of the team it went to. Whoever sees the request that asked it sees what was done in it, because sight goes down, and has no voice in it.

Sub-requests make a tree under the request first asked. A sub-request goes on when the run that asked it ends, and an answer that comes when nothing waits for it any more, from a sub-request or from a person, is kept in the request that asked, once, and makes it working again. An answer to a request leaves the sub-requests open beneath it going. Stopping a request stops everything still open beneath it, and nothing above it.

## Two layers

**What was done** is what leaves a team: what was said at the origin and to the asker, the sub-requests with what each asked and how each was answered, the decisions with what was decided, who decided and when, and the answers.

**What was read** belongs to one team: what its runs read and said, their tool results and conversations, and what was said between its workers and its people away from the origin, such as a worker's question to one of its people, or the reasons a decider of the team was shown. Bulldog keeps no copy of a run. What a run read is read from Corbie, by the run's labels, for as long as Corbie keeps it, and is of the team those labels name. A worker deciding for another team runs as the team it decides for, as Decisions says, so what it read there is that team's.

What a team with a disclosure check says outside it, an answer, a question or a tell to someone outside the team, or a sub-request it makes, is what was read of that team until it is let go. A disclosure refused is an entry by the decider, with its note, to the team's lead, and is what was read of the team. A refused answer starts the lead again on the same request; a refused question, tell or sub-request is given to the run that said it. Whoever asked the team is told nothing of the refusal but, when the lead gives up or the limit on runs in a row is reached, the one line Decisions gives: that the team did not disclose its answer, and who refused.

## Who sees what

One rule says who sees a request, read from the files each time something is shown or sent.

- **What was done** is seen by its asker, by the people of the team it went to, by the people who can read its origin, and, for a sub-request, by whoever sees the request that asked it. Sight goes down the tree, never up or sideways: Dana and Al see what payments was asked and how it answered, and Lee and Priya see payments' sub-request and what is beneath it, but not what Dana asked, nor what platform was asked beside it.
- **What was read** of a team is seen only by that team's people. Being in a team around it gives no sight inside it, and neither does seeing the request. A team that has gone, or has been renamed, keeps its past what was read for nobody.
- **An entry said to someone** is seen by them, with its answer, and nothing else of the request.

A team with no people has as its people, for every purpose of sight and control, the people its decisions reach, as the office gives them, out through the teams around it: they are told of its failures, see what it read and what was done of its requests, say more in them, and watch and stop its runs.

Saying more follows one rule, the same on the screens as through a bridge: the asker, the people of the team the request went to, and, for a request with an origin, the people who can read it, or, for a sub-request, the people and workers of the team that asked it. What they say is an entry to the team. Nobody else says more in a request, and seeing it from the request above it gives no voice in it.

A request someone may not see is, to them, a request that is not there: no number, no count, no notice, no worker shown busy on it. Nothing of a request is sent anywhere but to its origin and to those its entries are to, and what goes to the origin is only what was done.

## Guests

A guest is someone a bridge names who matches no person. A guest may ask only in the places the bridge's settings list as ones where guests may ask, a direct place like any other, and is answered there; which places those are is the bridge's. A guest may say more in what they asked, at its origin, and is put no question anywhere but there. A guest never decides, sees nothing on the screens and nothing of what was read, and anything done for them that needs a yes still waits for the team's deciders. When the office later names them, their identity matches a person, and what they asked is theirs to see.

## The store

Requests are kept in one store, in Bulldog's one file, and only this area reads or writes them. It keeps requests, their entries and the files on entries, and nothing of any other area. What waits on a person is worked out here, once, and the screens and the bridges read it here. A person can list the requests they may see, by team and by state, those that wait on them, and those they asked.

Each thing done is one write, whole or not at all: a request with its asking, an entry with its files, a sub-request with the entry that asked it, a stop with everything it closes. Whoever watches is told of a change after it is written. A store made by an older Bulldog is refused, and the person is told to make it again.

## Interfaces

- **Office** gives the teams with their people, leads and workers, and each person with the identities their bridges give, read each time sight is decided or a request is made; it checks that each team a bridge's place names is in the office, and gives the people a team's decisions reach, who stand as the people of a team with none.
- **Work** makes a routine's request, starts the lead on a request and again when it is working again, makes sub-requests through the tool Bulldog ships, gives an answer to the tool that waits for it and otherwise hands it here as a late entry, writes what its runs say outside a run as entries in their own request and no other, and reports how each run ended, telling a team with no people through the people this area names for it. It gives a run who is asking, as this area says, and a routine's run its team's open requests, each with its number, asker, state and first line, as the team's people see what was done. The labels it gives a run name the request and the team, and this area reads what was read from Corbie by them.
- **Decisions** writes each decision as an entry, with what is put to a person as an entry to them, says how a decision appears in what was done, holds what a team with a disclosure check says outside it back from what was done until it is let go, the answer of a lead who is a person included, which this area hands it as soon as it is kept, however it was said, and gives it to read every answer it holds, so that one Bulldog stopped before handing is handed then, and holds no longer, waiting on the lead again, when it cannot be handed, writes a refused disclosure as an entry to the team's lead, and, when the lead gives up or reaches the limit, tells whoever asked the team only that it did not disclose its answer, and who refused; says that a run asked to decide for a team is of that team, walks to whoever a team's decisions reach, whom this area makes the people of a team with none, and refuses a decision that reaches nobody, with why.
- **Bridges** hand in what is said, once, by the service's own id for it, with the sender's checked identity, the place it was said and the identities that can read it; say, for each place they hear, which team is asked there and whether guests may ask there; hear the status of a thing at its source and report it here, where it is shown, a close included; carry what is said to an origin or to a person; and read here what waits on whom and who may say more.
- **Changes** keeps each draft with the request it is drafted in, and a change's decision is an entry there like any other.
- **Server** keeps the file this store is in, and gives the screens each person's requests, both layers as they may see them, and what waits on them; who may say more and who are the people of a team with none it reads here, and who may stop or decide from the screens is its to say.
