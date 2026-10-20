# Requests: rules

The detailed rules behind the [Requests overview](../requests.md). Requirements are `R-requests-nn`, constraints `C-requests-nn`. The checks use the two offices of the README: the house (Sam and Kit; the steward, its lead, with the fixer, sparks, the clerk, bins and the builder; Sam decides everything) with the garden inside it (Sam its one person; the gardener, its lead, and the groundsman; Sam decides everything), and Platform support (Al and the bot in the office; platform, which is the ops worker alone; payments, which is Lee and Priya with the ledger and a disclosure check; support, which is Dana; and Raj, a person written in the office who is named in no team). A "stand-in bridge" is a bridge built for the tests, which hands in what it is told to, from a place its settings list, with the identity it is told to.

## A request

**R-requests-01** Something asked on a bridge, in the chat or by a routine makes a request, which holds a number, the team it went to, its asker, its origin, and a first entry that is the asking.
*Accept:* Kit says "the kitchen socket sparked" in the chat to the house. One request is kept, with a new number, the house as its team, Kit as she is signed in as its asker, the chat as its origin, and one entry, by Kit to the house, holding the words.

**R-requests-02** A request's number is given once in the office and never again, a sub-request's included, and it is the number every label and screen uses for it.
*Accept:* three requests are asked and one of them makes a sub-request: the four have four numbers. The server is restarted and a fifth request is asked: its number is none of the four.

**R-requests-03** A request goes to exactly one team, named when it is made: a request from a bridge goes to the team the place it was said in names in the bridge's settings, or to the office where the place names none; from the chat, to the team the person picks, or the office; from a routine, to its team; and a sub-request, to the team asked.
*Accept:* a request the stand-in bridge hands in from a place listed with no team goes to the house, and to the steward, its lead; one from a place listed as the garden's goes to the garden, and to the gardener. One said in the chat with no team picked goes to the house; one with the garden picked goes to the garden. The garden's Friday routine makes a request to the garden.

**R-requests-04** A request goes to its team's lead, or, when a routine makes it, to the worker the routine names. A lead that is a worker starts on what its team's heed lets through, where one weighs what is said (R-work-49).
*Accept:* Kit's request goes to the steward. The house's Monday routine, which names the steward, goes to the steward; a garden routine naming the groundsman, who is not the garden's lead, goes to the groundsman.

**R-requests-05** A request to a team whose lead is a person waits on that person, and what they say in it is its answer.
*Accept:* the bot makes a sub-request of support. It is waiting on Dana. Dana answers it on the screens, and the sub-request is answered with her words, by Dana.

**C-requests-01** A request is made only for a team that is in the office and has a lead that can answer for it; otherwise it is refused where it was asked, saying why, and nothing is kept. Otherwise it would wait on nothing without saying so. *Test:* `requests_team_with_no_lead_refused`

## The asker

**R-requests-06** A request's asker is the identity the bridge checked for whoever asked, or, for the chat, the person signed in; for a routine, it is the team.
*Accept:* Dana's message in #help is handed in with her Slack identity: the request's asker is that identity on the Slack bridge. The house's Monday routine has the house as its asker.

**R-requests-07** Who an identity is, is read from the office each time it is needed; one that matches no person is shown as a guest, with the bridge and the identity it vouched for.
*Accept:* Dana's Slack identity is taken off her in the office, and the office is read again. Her request in #help shows its asker as a guest, and Dana no longer sees it as its asker. The identity is written back, and she sees it again.

**C-requests-02** An asker, and who an entry is by or to, is kept as what it is (a checked identity, a person as signed in, a worker of a team, a team, a rule, the routine or Bulldog), never as a bare name. A person removed and a new one written under the same name would otherwise inherit the old one's requests, and a worker named like a person, or like Bulldog, would be taken for them. *Test:* `requests_reused_name_inherits_nothing`

**R-requests-44** A run given a request is told who is asking, as the office is read then: a person, by their name and the teams they are in; a guest, as a guest, with the bridge and the identity it vouched for; for a sub-request, the worker that asked and its team; for a routine, the team.
*Accept:* the bot's run on Dana's request is told that Dana asks, and that she is in support. Someone the office does not name asks in #help: the bot's run is told a guest asks, on `slack`, as their Slack identity. The ledger's run on payments' sub-request is told the bot of the office asks.

**C-requests-30** Bulldog judges nothing of what a guest may have. What a guest asks reaches the team's lead as anyone's request does, told that a guest asks, and anything done for them that needs a yes waits on the team's deciders as for anyone. Whether a stranger may have something is the team's to judge, with the machines it already has; a second scheme of Bulldog's beside them would be one more place to be wrong. *Test:* `requests_guest_judged_by_the_team`

## The origin

**R-requests-08** A request asked at a place keeps that place as its origin: the bridge and the place on it, as the bridge names the place, a channel, a thread or an issue, or the built-in chat; a routine's request keeps the routine that made it, and the person who ran it now, when one did; and every answer of the request goes there. A sub-request is asked at no place and has no origin of its own (R-requests-25).
*Accept:* Dana's request keeps the Slack bridge and her thread in #help as its origin, and the bot's answer is handed to the Slack bridge for that thread. Kit's request keeps the chat, and its answer is shown in the chat. Payments' sub-request has no origin, and its answer is given to the bot's run.

**R-requests-09** The people who can read an origin on a bridge are the people whose identities its in machine last reported as reading that place; a thing handed in with no readers reports none, and leaves them as they were (R-bridges-16); a place it never reported is read by the asker alone. The chat is read by the asker alone; a routine by the people of the routine's team.
*Accept:* the stand-in bridge reports Dana's, Al's and Raj's Slack identities as reading #help: all three see what was done of Dana's request. A later thing from #help reports Dana and Al alone: Raj no longer sees it. A thread in a place the bridge never reported readers for is read by its asker alone. Kit's chat request is read by Kit alone. A garden routine's request is read by the garden's people.

**R-requests-10** Whatever is said to the asker of a request that has an origin goes there, as its answers do. Nothing the asker says goes back to them: a routine's asking, its say, which its team says to itself, goes to nobody.
*Accept:* the fixer asks Kit "tell me when it arrives". The question is an entry to Kit, and is shown in the chat where Kit asked. In Dana's request, a question the bot puts to Dana is handed to the Slack bridge for her thread. The house's Monday routine's asking is carried to neither Sam nor Kit, and when the steward answers it with nothing, nothing of it is carried at all.

## Entries

**R-requests-11** Everything said in a request is an entry, which holds the request, who it is by, who it is to, when, its words, any files, where it came in (a bridge's place or the screens), and its layer.
*Accept:* Lee's yes to the refund, given on Slack, is an entry in payments' sub-request, by Lee's Slack identity, to the ledger, at the time it came, marked as having come in on the Slack bridge's direct message to Lee.

**R-requests-12** Entries are kept in the order they were written, and an entry is never changed or removed.
*Accept:* five entries are written to a request; it is read back with the five in that order. There is no way, from any area or from the screens, to edit or remove one.

**R-requests-13** An entry may carry files, kept with it in the store, and a file is seen by whoever sees its entry and by nobody else.
*Accept:* Lee says something to the ledger in payments' sub-request, with a statement attached. The entry holds the statement; Lee and Priya see it; Al does not, and his asking for it by its name is answered as asking for nothing.

**R-requests-14** Bulldog's own entries say that a run failed, that a request was stopped and by whom, or a line of Bulldog's own in place of who (R-requests-21), that its thing was closed at the source and by whom, and that a team or a person has gone from the office; they are by Bulldog. Of a failure, the entry to the asker says only that the team could not answer, and the error's words are in an entry to the team, which is what was read of it. A team with no people is told through the people its decisions reach (R-requests-36).
*Accept:* the steward's run fails with an error. The request gains an entry by Bulldog to Kit, its asker, saying that the house could not answer and holding none of the error's words, and an entry by Bulldog to the house, of what was read of the house, holding the error's words.

**C-requests-04** Words are kept whole: a text or a file larger than the store takes is refused whole, saying the limit, and never kept cut short. What is cut short silently is taken for what was said. *Test:* `requests_too_large_refused_never_cut`

**C-requests-05** An entry and its files are written together or not at all, so a refused entry leaves no file behind. A file written before its entry is refused would be kept with nobody's entry to see it by. *Test:* `requests_refused_entry_leaves_no_file`

**C-requests-06** Times are kept as instants and shown in the office's zone, the `zone` of the top team file. Times in one zone beside others in another are misread. *Test:* `requests_times_in_office_zone`

## States

**R-requests-16** A request whose origin keeps no status of its own is working while its team is on it and nothing in it waits on a person; waiting on someone while something in it waits on a person; and answered once its lead has answered, or its team's heed noted what was said (R-requests-47), and nothing has been said in it since.
An empty answer to a request asked by a routine, or at a channel place, any place on a bridge but its direct place, says nothing: the request is answered with no entry, and nothing goes to the team's people or to the place. An empty answer to any other request is an entry like any answer. An answered request is labelled by one word at most: stopped, while it was stopped and nothing since has opened it; noted (R-requests-47); or quiet, while the last message in it not noted has no answer after it, its lead having answered it with nothing.
*Accept:* Kit's request is working while the steward's run goes on. When the fixer asks Kit, it is waiting on Kit. When Kit answers, it is working. When the steward answers, it is answered. The steward answers the house's Monday routine with nothing: the request is answered and quiet, and its thread holds only its asking. Kit says "thanks" in the garden's Telegram chat, and the gardener answers with nothing: the same. The steward answers Kit's Telegram direct message with nothing: that answer is an entry, and the request is not quiet. A noted request Sam stops is stopped.

**R-requests-47** A team's heed that notes messages in a request, the asking or more said, writes one entry for each message it noted: by the rule, as of the team, to the team, answering the message, holding the rule's note, or that the rule noted it when it gives none. The entry is what was read of the team, and goes to nobody. The message is never changed, and whoever sees it sees that it was noted; only the team's people see the rule and its note. The request is answered with nothing said, and a stopped one is left as it is. A message noted already is not noted again, and only a message is noted. An answered request is labelled Noted while nothing in it was ever answered, said or quiet: every message in it was noted, and nothing in it answers its asker. A Noted request waits on nobody. A request its lead answered, with words or with nothing, stays answered and is not Noted when later talk in it is noted.
*Accept:* Kit says "lovely weather" in the garden's Telegram chat, and the garden's heed, `garden/for-us.js`, notes it, "Not for the garden": the request is answered and Noted, nothing waits on Sam, and nothing is carried anywhere. Kit sees her words, marked noted, and not the heed's entry; Sam sees both. Noted again, nothing more is written. Kit says "@gardener the roses?": the request is working, and not Noted; the gardener answers "Prune in March.": it is answered, and not Noted. Kit says "thanks", which the heed notes: the request is answered, not Noted, and "thanks" is marked noted. In another request there, the gardener answers "thanks @gardener" with nothing, and Kit's "cheers" after it is noted: answered, and not Noted. A Noted request Sam stops is answered with its stop, and not Noted.

**R-requests-17** Waiting on someone names everyone it waits on: the person a question is put to, the people a decision has reached, or the person who is its lead.
*Accept:* the refund is put to Lee and Priya: payments' sub-request is waiting on Lee or Priya. Lee says yes: it is working.

**R-requests-18** What waits on a person is every open entry to them in a request they may see: a question they have not answered, a decision put to them that nobody has made, and a request they are the lead of that is not answered. A Noted request (R-requests-47) waits on nobody: its lead never started on it.
*Accept:* while the refund waits, what waits on Lee is that decision, and the same for Priya. Priya decides it: nothing waits on either. A sub-request to support waits on Dana until she answers it.

**C-requests-07** What waits on a person is worked out in one place, and the screens' list, their count, and what bridges send all read it there. Written in more than one place, the copies drift, and a person is told of something their list does not show. *Test:* `requests_waiting_on_agrees_everywhere`

**R-requests-19** May say more in a request: its asker; the people of the team it went to; for a request with an origin, the people who can read it; and for a sub-request, the people of the team that asked it and, through their runs, its workers. It is the same on the screens as through a bridge. What they say is an entry to the request's team, whether the request is open or answered, and makes an answered request working again. Nobody else may, and seeing a request from the one above it gives no voice in it.
*Accept:* after the steward answers, Kit says "it sparked again". The request is working, and its last entry is by Kit to the house. Raj, who reads #help, says more in Dana's thread there: it is an entry by Raj's Slack identity to the office; he says more on the screens: it is an entry by Raj to the office. After payments answers, Al, the office's person, says more in payments' sub-request on the screens: it is an entry by Al to payments, and the sub-request is working. Dana and Raj, who see payments' sub-request from Dana's request, cannot say more in it, on the screens or anywhere.

**R-requests-20** A run that ends with no answer, because it failed or was stopped, while nothing else in its request is open, leaves the request answered, with Bulldog's entry to the asker, the request's last, saying what happened, which goes to the origin; of a failure, that entry says only that the team could not answer. Saying more makes it working again.
*Accept:* the steward's run fails while it waits on nothing. The request is answered; its last entry, by Bulldog to Kit, says that the house could not answer, holds none of the error's words, and is shown in the chat where Kit asked. The error's words are only in Bulldog's entry to the house. Kit says "try again": the request is working.

**C-requests-09** A request that was stopped is opened again only by someone who may say more in it saying more; nothing said after the stop by a run the stop cancelled opens it. A run cancelled in Corbie can still say something before it ends, and a stop it undid would be no stop. *Test:* `requests_stopped_opened_only_by_a_person`

**R-requests-21** Stopping a request stops every sub-request still open beneath it, closes the questions and decisions open in them, and answers each with Bulldog's entry naming who stopped it. Bulldog may stop a request with a line of its own in place of its name, as when a team did not disclose its answer: the request's asker is told that line alone, and what is stopped beneath it is answered "stopped by Bulldog".
*Accept:* while the refund waits on Lee, Al stops Dana's request. Payments' and platform's sub-requests are answered "stopped by Al", nothing waits on Lee or Priya any more, and Dana's request is answered "stopped by Al". Bulldog stops Dana's request with a line of its own: Bulldog's one entry to Dana is that line, the question to Lee is closed, and payments' and platform's sub-requests are answered "stopped by Bulldog".

**R-requests-22** Stopping a sub-request stops it and what is beneath it, and nothing above it; that it was stopped, and by whom, is its answer.
*Accept:* Lee stops payments' sub-request. It is answered "stopped by Lee"; Dana's request is still working, and "stopped by Lee" is payments' answer to the bot's question.

## Status at the source

**R-requests-23** A request from something that keeps a status of its own holds one status for it, the one its bridge's in machine last reported, with when, and shows it beside whether its team's work on it is going or waits on someone; a status reported is not an entry, and starts nothing.
*Accept:* the stand-in bridge hands in GitLab issue 51 as opened, and later as "in review" while the bot is on it. The request shows "in review" as GitLab's, with the time it was reported, and beside it that its work is going. The run goes on; the request gains no entry, and nobody is told.

**R-requests-45** The thing being closed at its source stops the request's work as a stop does, with Bulldog's entry saying who closed it there, by the identity the bridge gave; a later status, a reopening included, starts nothing, and something said in it starts it again.
*Accept:* while the bot is on issue 51 and platform's sub-request is open, the stand-in bridge reports it closed by Al's GitLab identity. The bot's run and platform's sub-request are stopped, and the request's last entry is by Bulldog, "closed at gitlab by Al", the bridge named by its folder, as the office knows it, and beside "closed" nothing is shown going or waiting. The bridge then reports it reopened: nothing starts. Al comments on the issue: the request is working again.

**C-requests-31** Bulldog never sets a source's status, there or here: a request from an issue shows only what its in machine reported, and neither an answer, a stop nor anything else in Bulldog changes it. A status with two writers is wrong in one of the two places, and the source is where its own people look. *Test:* `requests_source_status_never_set`

## Sub-requests

**R-requests-25** A run of one team asks another through the tool Bulldog ships, and that makes a sub-request: a request to the team asked, whose asker is the worker that asked, which is asked at no place but has the request it was asked from, and whose first entry is the question; and the asking request gains an entry, by that worker to that team, that leads to it.
*Accept:* the bot asks payments to refund one charge. A sub-request is made to payments, asked by the bot, with no origin, Dana's request as the request it was asked from, and the bot's question as its first entry; Dana's request has an entry by the bot to payments that leads to it.

**C-requests-10** A sub-request is kept with the Corbie ticket its asking run waits on, and a ticket that already has one makes no second. After a restart Corbie shows every waiting call again, and each would otherwise make the sub-request again. *Test:* `requests_one_sub_request_per_ticket`

**R-requests-26** A sub-request keeps the origin at the top of its tree, so the team asked can be told who will in the end read what it says.
*Accept:* payments' sub-request gives #help on Slack as the origin at the top. A sub-request that payments makes of platform gives the same.

**R-requests-27** A sub-request's answer is an entry in it by its lead, once any disclosure check of its team has let it go, and is given to the run that asked; in the asking request it is what was done.
*Accept:* the ledger answers "Duplicate confirmed; £40 refunded" and the disclosure rule lets it go. Payments' sub-request is answered with it; the bot's run is given it; Dana and Al see it in Dana's request.

**R-requests-28** A sub-request stays as it is when the run that asked it ends, unless that run was stopped.
*Accept:* the bot's run is cancelled in Corbie while the refund waits on Lee. Payments' sub-request is still waiting on Lee or Priya.

**R-requests-29** An answer that comes when no run waits for it, from a sub-request or from a person asked a question, is kept as an entry in the asking request, to the worker that asked, and makes that request working. A sub-request's answer is kept so once: handed in again, as after a restart, it writes nothing more.
*Accept:* the fixer asks Kit to say when the socket arrives, and the fixer's run then fails. On Thursday Kit answers. Her answer is an entry to the fixer in the request, which is working again. Payments answers after the bot's run has answered Dana: its answer is one late entry in Dana's request however often it is handed in, and a later answer of payments is a second.

**C-requests-11** A late answer is never dropped, a worker's no more than a person's. An answer dropped is work lost, and the run that made it is spent for nothing. *Test:* `requests_late_answer_kept`

**C-requests-29** Answering a request leaves the sub-requests still open beneath it going, and their answers come back as late answers; only a stop closes what is beneath. A team that answers early has not withdrawn what it asked, and an answer still to come is work already paid for. *Test:* `requests_answer_leaves_sub_requests_going`

**R-requests-30** A request lists its sub-requests, each its own, as far down as they go, with each one's team, question, state and answer.
*Accept:* Dana's request lists payments' and platform's sub-requests, with their states; when payments asks platform, that sub-request is listed beneath payments'.

## Two layers

**R-requests-31** An entry is of what was done when it is said at the origin, by or to the asker, or across a team's wall (a sub-request's question, its answer, a question or a tell to someone outside the team, the record of a decision); and it is of what was read of a team when it is said between that team's own people and workers away from the origin. What crosses the wall out of a team with a disclosure check is what was read of that team until the check lets it go.
*Accept:* Al asks something himself in #help: his entry is what was done. The bot asks Al in a direct message: what was read of the office. The ledger's reasons, as Lee is shown them: what was read of payments. "Refund one charge: approved by Lee, 14:02": what was done of payments' sub-request. Payments asks platform something: until the disclosure rule lets the question go, it is what was read of payments, and Al and Dana see nothing of platform's sub-request.

**R-requests-32** A decision in what was done shows the action as its worker's definition describes it, what was decided, who decided and when.
*Accept:* Dana's request shows, under payments' sub-request, "Refund one charge: approved by Lee, 14:02". Kit's request shows the fixer's cutting of the circuit as "Turn one circuit off: approved by Sam".

**C-requests-13** What was done never holds an action's arguments or the reasons a decider was shown. They are what the team read: the card numbers in the refund. *Test:* `requests_decision_shows_no_arguments`

**R-requests-33** Whatever a team's disclosure check holds, an answer, a sub-request it makes, or a question or a tell that anyone outside the team reads (R-work-13), is what was read of that team until it is let go, and what is refused never becomes what was done. A held answer is not yet the request's answer: until it is let go the request is working, or waiting on the people the disclosure has reached.
*Accept:* the disclosure rule refers an answer to Lee. Until Lee lets it go, Al and Dana see payments' sub-request waiting on Lee or Priya, and not the answer. Lee refuses it: it is never shown to them.

**R-requests-46** A refused disclosure is an entry by the decider, with its note, to the team's lead, always, and is what was read of that team. A refused answer makes the request working, and the lead is started again on it; a refused question, tell or sub-request is given to the run that said it, which goes on. Whoever asked the team is told nothing of it but, when the lead gives up or the limit on runs in a row is reached, the one line Decisions gives, that the team did not disclose its answer, with who refused (R-decisions-40).
*Accept:* Lee refuses the ledger's answer with "no card numbers in an answer". Payments' sub-request gains an entry by Lee to the ledger holding the note, seen by Lee and Priya alone, and is working; the ledger is started on it again. The bot's run is given nothing, and Dana and Al see payments' sub-request working, with nothing of the refusal.

**R-requests-34** What was read of a run is read from Corbie, by the labels that name its request and its team, and is of the team they name, for as long as Corbie keeps the record; when Corbie has dropped it, the request says so. A worker deciding for another team runs as the team it decides for, as Decisions says, so what it read there is of that team.
*Accept:* Lee opens what was read in payments' sub-request and sees the ledger's conversation and tool results from Corbie's record. Corbie's record is removed: the same view says the record of that run is no longer kept. For this check, payments' `decide` names the bot for the refund: what the bot read deciding it is seen by Lee and Priya, and not by Al.

**C-requests-14** No copy of a run's record is kept. Corbie keeps it, sealed; a copy would be a second place for it to leak from and to fall out of date. *Test:* `requests_no_copy_of_runs`

## Who sees what

**R-requests-35** What was done of a request is seen by its asker, by the people of the team it went to, by the people who can read its origin, and, for a sub-request, by whoever sees the request it was asked from.
*Accept:* in the work day, Dana (the asker), Al (the office's person) and Raj (reported reading #help) see Dana's request and both sub-requests; Lee and Priya see payments' sub-request and not Dana's request or platform's; nobody else does.

**R-requests-36** What was read of a team is seen only by that team's people. A team with no people has as its people, for every purpose of sight and control, the people its decisions reach, as the office gives them (R-decisions-39): they are told of its failures, see what it read and what was done of its requests, say more in them, and watch and stop its runs.
*Accept:* Lee and Priya see the ledger's conversation and the reasons put to them. Al, who runs the office, sees neither; nor does Dana. Platform has no people, and its decisions reach Al: Al sees what the ops worker read in platform's sub-request, and Dana does not; the ops worker's run fails, and Bulldog's entry to platform holding the error is seen by Al, and not by Dana.

**R-requests-37** An entry said to someone is seen by them, with what answers it, and gives them sight of nothing else in the request.
*Accept:* for this check, payments' `decide` names Raj for the refund, and the stand-in bridge says #help is read by Dana and Al alone. The refund is put to Raj. Raj sees that decision, what a decider is shown of it, and his own answer, and nothing else of payments' sub-request or of Dana's request.

**C-requests-15** Being a person of a team around another gives no sight inside it; only being its person does, or, for a team with no people, being reached by its decisions. Teams are sealed, and the simpler rule is the one that is kept. *Test:* `requests_no_sight_inside_a_team_within`

**C-requests-16** Sight goes down the tree, never up or sideways: whoever sees a request sees what was done beneath it, and a team asked something sees its own sub-request and what is beneath it, not the request it was asked from nor the other sub-requests beside it. A team is told what it is asked, and the rest is its asker's. *Test:* `requests_sight_goes_down_never_up`

**C-requests-17** Sight is read from the files each time anything is shown or sent: one taken out of a team is shown and sent nothing more of it, and one put back sees it again, earlier requests included. A notice sent by who was in a team when it was written reaches people who have left. *Test:* `requests_sight_follows_the_files`

**C-requests-18** A request someone may not see is, to them, the same as one that is not there: the same answer as for a number never given, and no number, count, change notice or worker shown busy on it. A refusal that differs, or a number in a change notice, tells someone that a request they may not see exists. *Test:* `requests_unseen_same_as_absent`

**C-requests-19** Nothing of a request is sent anywhere but to its origin and to those its entries are to, and what goes to the origin is only what was done. Whoever can read a place reads what is sent there, so a notice in a wider place lets out what a team read. *Test:* `requests_nothing_leaks_through_a_notice`

**C-requests-20** When whom something is for cannot be found, it is refused, saying so, to whoever said it, and never given to someone else instead. Whoever is picked instead reads what was not for them. *Test:* `requests_no_fallback_recipient`

**R-requests-38** When the office is read, a question to a person who can no longer see the request it is in, because they have gone from the office or been taken out of its team, is answered by Bulldog saying so; and a decision put to a person who has gone, or who is no longer on the list that put it to them, walks on past them to whoever comes after them.
*Accept:* the fixer asks Sam a question in Kit's request, and Sam is taken out of the office. When the office is read again, the question is answered by Bulldog "Sam is no longer in the office", and the request is working. Sam is written back in the office but not in the house, and the steward asks him again: at the next reading, that question is answered "Sam is no longer in the house". The refund is put to Lee and Priya, and Lee is taken off payments' `decide` list while staying in payments: at the next reading it waits on Priya alone, and nothing waits on Lee.

**R-requests-39** A request to a team that has gone from the office, by being removed or renamed, keeps what was done for those who still see it, is answered by Bulldog saying the team has gone if it was open, and what was read of that team is seen by nobody.
*Accept:* Sam asks the garden something in the chat, and the garden folder is removed while the request is open. The request is answered "the garden is no longer in the office"; Sam, its asker, still sees what was done of it, and no longer sees what was read in it, which nobody sees; Kit, in the house and not in the garden, saw nothing of it before and sees nothing now. Another request to the garden is answered, and the folder is then renamed `yard`, with Sam its person: nobody sees what was read in it, Sam as yard's person included.

**C-requests-21** What was read of a team that has gone falls to nobody, never to the team around it. Sight never walks up, a team's leaving included. *Test:* `requests_gone_team_read_by_nobody`

**C-requests-28** A team is written on its requests as gone at the reading that finds it gone, so a team written later under the same folder name is a new team, and inherits none of the old one's requests and no sight of what was read in them. A team known by its folder's name alone hands a newcomer everything its namesake was told. *Test:* `requests_reused_team_name_inherits_nothing`

## Guests

**R-requests-40** Something handed in by a bridge, from a place its settings list as one where guests may ask, with an identity that matches no person makes a request whose asker is that guest, to the team the place names, answered at its origin; from a place listed without guests, or from a place its settings do not list, it makes nothing. A direct place is listed like any other, and which places take guests is the bridge's settings' to say.
*Accept:* the work office lists only #help as a Slack place, for the office, with guests. Someone the office does not name writes in #help. A request is made to the office with them as a guest asker, and the bot's answer is handed to the Slack bridge for their thread. The same identity writes a direct message to the Slack bot: it is not heard, because #help is the only Slack place listed, and no request is made. A GitLab identity that matches no person comments on an issue in platform/api, which the GitLab bridge lists for platform without guests: no request is made.

**R-requests-41** A guest may say more at the origin, which is an entry by them in the request, and a question to a guest asker is put at the origin.
*Accept:* the bot asks the guest "which customer?". The question is handed to the Slack bridge for their thread; the guest's reply there is an entry by the guest to the office.

**R-requests-42** A guest whose identity the office later gives to a person is that person from then on, and sees what they asked.
*Accept:* the guest's Slack identity is written for a new person, Jo, and the office is read again. Jo signs in and sees the request the guest asked, as its asker.

**C-requests-22** A guest never decides, is put no question anywhere but at the origin, and is shown nothing on the screens and nothing of what was read. Someone reached only by one place must not be waited on anywhere else, and is shown nothing they have not asked. *Test:* `requests_guest_asks_and_nothing_more`

## The store

**R-requests-43** A person can list the requests they may see, by team, by state, and by the bridge they were asked on and the place it lists that they were asked at; the requests waiting on them; and the requests they asked.
*Accept:* Sam lists the house's requests by state and sees Kit's request as answered and Monday's routine as working; his list of what waits on him holds the circuit to cut; Kit's list of what she asked holds her request. Kit then says hello to the house directly on Telegram and asks of the hedge in `garden-chat`: Sam's list of what was asked on Telegram holds both, and of `garden-chat` the hedge alone.

**C-requests-23** Only this area reads or writes the requests' part of the store, and every other area asks it. Queries written elsewhere against its tables drift from each other and from the store. *Test:* none: it is a rule of the code's shape, kept by review.

**C-requests-24** Each thing done is one write, whole or not at all: a request with its asking, a sub-request with the entry that leads to it, an entry with its files, a stop with everything it closes. A failure part of the way through would leave a request begun with nothing in it. *Test:* `requests_one_thing_one_write`

**C-requests-25** Whoever watches is told of a change only after it is written, once for each request it changed. Told before, a watcher reads half a change or waits on the writer. *Test:* `requests_told_after_written`

**C-requests-26** A failure of the store is reported as a failure, never as a request not there or one not to be seen. A failure reported as a refusal hides what is wrong from the person who could fix it. *Test:* `requests_failure_not_a_refusal`

## Interfaces

- **Office** gives the teams with their people, leads, `decide` lists and routines, and each person with the identities their bridges give, and checks that each team a bridge's place names is in the office, and gives the people a team's decisions reach, who stand as the people of a team with none (R-requests-36); a reading that finds a person, a team or a decider gone, a team renamed included, closes here what waited on them (R-requests-38, R-requests-39, C-requests-28).
- **Work** makes a routine's request, and is told the one it opened that is still open, and the sub-requests asked through the tool Bulldog ships (C-requests-10), tells a run who is asking (R-requests-44) and the requests above its own, top first (R-work-20, C-work-06), gives a routine's run its team's open requests, each with its number, asker, state and first line as the team's people see what was done, gives a run what was said before its request where it was asked, of the requests every reader of it reads (R-work-52, C-work-61), writes what a run says in its own request and no other, writes what a team's heed noted (R-requests-47), cancels the runs of a stop whose closing is this area's (R-requests-21, R-requests-45), and writes Bulldog's entries for a run that failed (R-requests-14), to a team with no people through the people of R-requests-36; the labels it gives a run name the request and the team, and what was read is read from Corbie by them (R-requests-34).
- **Decisions** writes each decision as an entry, each naming the decision it is of, holds what a disclosure check holds back from what was done until it is let go (R-requests-33), the answer of a lead who is a person included, which this area hands it as soon as it is kept, on the screens or through a bridge alike, and holds no longer, waiting on the lead again, when it cannot be handed, writes a refused disclosure as an entry to the team's lead and gives the one line whoever asked the team is told when the lead gives up or reaches the limit (R-requests-46), says that a run asked to decide for a team is of that team (R-requests-34), walks to whoever a team's decisions reach, whom this area makes the people of a team with none (R-requests-36), and refuses, with why, a decision that reaches nobody, a decider's going included (R-requests-38).
- **Bridges** hand in what is said, with the sender's checked identity, its place, whether it names the bridge's own bot, which its entry keeps (R-bridges-41), the source thing it is about, which a request its asking opens keeps with its origin, and the identities that can read it (R-requests-09), once for each thing heard; say, for each place they hear, which team is asked there and whether guests may ask there (R-requests-03, R-requests-40); hear a source's status, which is shown here, and its closing, which stops the request's work (R-requests-23, R-requests-45); carry out what goes to an origin or to a person; and read here who may say more (R-requests-19).
- **Changes** keeps each draft with the request it is drafted in, and a change's decision there is an entry like any other.
- **Server** opens the store this area keeps its part of, and refuses one made by an older Bulldog; it gives the screens each person's requests, both layers as they may see them, and what waits on them, with everyone they name as the office names them now (R-server-33), reads here who may say more (R-requests-19), who are the people of a team with none (R-requests-36) and the request a routine opened that is still open (R-server-32), and says who may stop a request.
