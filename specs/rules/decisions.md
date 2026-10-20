# Decisions: rules

The detailed rules behind the [Decisions overview](../decisions.md). Requirements are `R-decisions-nn`, constraints `C-decisions-nn`. "The house" and "the work office" are the offices in the README's day, with the team files of the overview; the house is the fuller example the office page shows, with its Telegram bridge, not the one written into an empty folder. "A stand-in screening service" is one built for the tests, answering as it is told to.

## What waits for a yes

**R-decisions-01** A job Corbie holds for a yes reaches Bulldog through the ask Bulldog ships, the nearest at or above the machine that holds it, and each held job becomes a decision of the team whose worker holds it.
*Accept:* in the house, the steward names the ask Bulldog ships as its `ask:`, and the fixer and sparks name none. The steward calls the fixer as a tool, and the fixer calls `cut_power`, whose job is marked for approval. The ask the steward names is started, and one decision waits, of the house, naming the fixer, `cut_power`, what the tool says it does, the circuit it would be given and what the model said with the call. The job has not started in Corbie. Written with the fixer naming the ask Bulldog ships itself, the fixer's ask is started instead, and the one decision that waits is the same, of the house, naming the fixer.

**R-decisions-02** A job held because its permission is listed under `ask_about:` is decided exactly as one marked for approval.
*Accept:* a worker lists `net:payments.example` under `ask_about:` and calls a tool that declares it. One decision waits for it, with the same parts as in R-decisions-01, and Corbie starts the job only after a yes.

**R-decisions-03** Several jobs held from one turn are each their own decision, and the ask's ticket is answered once, when every one of them is decided, naming the jobs approved and joining the notes.
*Accept:* a worker's turn asks for two held tools. Two decisions wait. A yes on the first answers nothing in Corbie; a refusal with the note "not today" on the second answers the ticket once. Corbie runs the first job, and the model is told the second was denied, with "not today".

**R-decisions-04** A team that sets `disclosure` has everything it says to anyone outside it held until it is disclosed, and it goes on only then: an answer, a question or a tell that anyone outside it reads, and a sub-request it makes. What is said to the asker is read by them and, at a place on a bridge, by everyone the bridge last reported as reading the place, someone the office cannot name counting as outside, or, where it never reported any, by any guest at all at a place guests may ask in, but a direct place, and by the asker alone at any other; anything else is read by whom it is said to. So at a place guests read, or may read for all the bridge has said, every answer of a team that sets one waits on it, whoever its lead is.
*Accept:* in the work office, payments answers a sub-request from the bot. Until the disclosure decision ends, the bot's run is still waiting and the office's request shows no answer from payments. The ledger asks platform something with `ask_team`: until that disclosure decision ends, no sub-request to platform is made. A team with no `disclosure` has its answer go on at once, and so does payments' answer to Lee in the chat. With Lee as payments' lead and a place #refunds naming payments, Lee's answer to Priya in a thread the bridge reports only payments reading goes on at once, and so does one in a thread whose readers it never reported; in a thread that someone the office cannot name reads too, it is held, and so it is in an unreported thread of #cards, a place naming payments where guests may ask.

**R-decisions-05** A change to the office files, once put, is a decision of the top team, whether a worker drafted it or a person on the screens. Writing a team's notes is not one: it is the one write to the office's files that is not a decided change, and is still a commit that says who wrote it and why (R-changes-34).
*Accept:* the builder drafts a change and puts it to be decided. One decision waits, of the top team, of the kind change, showing who drafted it, why, what it changes and what it lets more by. A worker whose definition names the notes tool Bulldog ships writes one of its team's notes: no decision waits, and the history's newest commit names that worker and its why.

## Which list

**R-decisions-06** A held action is decided by the team's `decide` list named for the held job, or for a listed tool's call by the list named for its entry when none is named for the job, and by `everything` when neither is.
*Accept:* in the house with the reviewer added, `order_part` is decided by `[./small-orders.js, sam]` and `cut_power` by `[reviewer, sam]`; a held `open_door`, which the house names no list for, is decided by `everything`, and Sam is asked. With the fixer given a `github` entry, a held `github_create_issue` whose call names the entry `github` is decided by `github: [kit]`, and by `github_create_issue: [kit]` when `github: [sam]` is written too.

**R-decisions-07** A disclosure is decided by the team's `disclosure` list, and never by a list named for a held job; a change is decided by the list the changes area names for it (R-changes-17), walked as any other.
*Accept:* payments' answer is put to `./no-account-numbers.yaml` first, and the ledger's held refund never is. A change put in the work office is walked over the list R-changes-17 gives, with the same referrals, entries and final answer as a held action.

**R-decisions-08** A team that writes no `everything` list has its own people as that list.
*Accept:* payments, written without its `decide`, has the ledger's held refund asked of Lee and Priya at once.

## The walk

**R-decisions-09** Deciders are asked one at a time, in the order of the list, until a person is reached; each rule or worker says yes, refuses or refers, and a referral goes to the next, carrying its note.
*Accept:* a list `[./a.js, ./b.js, sam]` where `a.js` refers with "unsure, 0.6" and `b.js` refuses: `a.js` is run, then `b.js`, whose input's `referrals` hold the referral of `a.js` with its note, and Sam is never asked. The thread holds "referred by a.js: unsure, 0.6", an entry of the kind `referred`, and "refused by b.js: no plumber today", each with its note.

**R-decisions-10** A refusal from any decider ends the decision as refused, whoever comes after it.
*Accept:* with `cut_power: [reviewer, sam]`, the reviewer refuses with a note. The decision is refused, Sam is not asked, and the fixer's model is told it was denied, with the reviewer's note.

**R-decisions-11** A yes from any decider ends the decision as yes, whoever comes after it in the list. A rule or worker that wants a person to look refers instead, with its view as its note, and the person is shown it.
*Accept:* with `order_part: [./small-orders.js, sam]`, the rule says yes to a £12 order; the part is ordered and Sam is not asked. It refers a £300 order with "£300 is more than I order alone."; Sam is asked, shown the rule's referral and its note, and the job waits until Sam answers. With `cut_power: [reviewer, sam]` the reviewer says yes with "breaker 4 is the kitchen": the job runs and Sam is not asked. The reviewer refers instead with "it may feed the freezer, 0.6": Sam is asked and shown that note.

**R-decisions-12** When the walk reaches the first person of a list, every person from there to the end of the list is asked at once, and the first to answer decides.
*Accept:* payments' `everything: [lee, priya]`: Lee and Priya are both sent a direct message at the same moment and both see the decision on the screens. Lee says yes; the decision is approved by Lee, and Priya's answer a minute later is refused, telling her Lee decided and when.

**R-decisions-13** A list in which every decider refers goes on to the team's `everything`. An `everything` in which every decider refers goes on to the `everything` of the team around it, and so does a team's that names nobody, since it writes none and has no people; and so on out to the office. A list may name rules and workers alone. The walk is the same given a team and the key of a list with no decision at all, as the server and `check` show who decides a tool (R-server-32, R-server-03).
*Accept:* with `order_part: [./small-orders.js]`, a list with no person, the rule refers a £300 order and Sam, the house's `everything`, is asked. Platform, written without its `decide`, has no people and so an empty `everything`: a held action of `ops` is asked of Al, the office's `everything`. Payments' `disclosure: [./no-account-numbers.yaml]` refers an answer naming an account number, and it goes on to Lee and Priya through payments' `everything`; it waits until Lee lets it go. Given the house and `order_part` with no decision, the walk is `small-orders.js`, then Sam; given platform and `everything`, it is Al, on the office's `everything`.

**R-decisions-14** Every walk that is not ended on the way ends with a person asked: people do not refer, and when every list a walk reaches refers, it comes to the office's `everything`, which reaches a person.
*Accept:* the garden writes `decide: { everything: [./a.js] }` and the house `decide: { everything: [reviewer, sam] }`. With `a.js` and the reviewer both referring, the gardener's held `open_door` is asked of Sam, and the thread holds both referrals, each with its note.

**R-decisions-39** The people a team's decisions reach are the people its `everything` walk comes to when every rule and worker on the way refers: those its own `everything` names, or, where it names none, those of the first `everything` outward that names one. They stand as the people of a team with no people, as the requests area says (R-requests-36).
*Accept:* platform, which has no people and no `decide`, has its decisions reach Al, the office's `everything`. Written `decide: { everything: [./a.js] }`, they still reach Al. In the house, the garden written `decide: { everything: [./a.js] }` with no people has its decisions reach Sam.

**R-decisions-38** A decision that nobody can be reached to decide is refused by Bulldog, its note saying why, and the worker is told as for any refusal; it is never left waiting.
*Accept:* a held job reaches Bulldog from a run labelled with the garden after a reading has found the garden gone, before Corbie has cancelled the run. The decision is refused by Bulldog with "nobody can decide this: garden is no longer in the office", naming the team by its folder, since what a team was called is not kept once it is gone, the thread holds that entry, and Corbie is answered that the job is denied, with that note.

**R-decisions-15** A person says yes or refuses, with an optional note, and does not refer.
*Accept:* the screens offer a person yes and refuse on a decision waiting on them, and a note. An answer from a bridge that is neither is not taken, and the person is told what they may answer.

**R-decisions-16** Anyone the walk would reach may decide a decision while it waits, before it reaches them, on the screens or by answering a direct message they were sent earlier on it.
*Accept:* with `order_part: [./small-orders.js, sam]` and a rule still running, Sam decides the waiting decision on the screens. Sam's decision stands, and the rule is cancelled. Kit, who is in the house but whom no list names, is shown the decision and cannot decide it.

## Rules

**R-decisions-17** A rule is in the team's folder, or is one Bulldog ships, named in a list by its path: a bare script, which needs nothing and decides from its input, or a Corbie definition, which holds its own allow lines. Bulldog starts either in Corbie with the decision as its input, and its result is `{ says, note }`, where `says` is `yes`, `refuse` or `refer`. Whichever folder holds it, a rule decides as the team whose decision it is: its run is labelled with that team (R-work-47), and its word in the thread is by the rule as of that team and is that team's reading. A rule a team names as its heed (R-office-36) is read and checked as any rule, and is no decider: no decision is asked of it.
*Accept:* `small-orders.js` is run as a run of the office with the decision's labels. Its input holds the kind, the team, the request, the action with its name, description and arguments, what the worker said, the referrals so far as `referrals` and where a referral would go as `referral_goes_to`. It returns `{ says: "yes", note: "Under £20." }`, and the thread holds "approved by small-orders.js: Under £20.". `no-account-numbers.yaml` is started the same way, with the same labels and a disclosure decision as its input, and its result is read the same way.

**R-decisions-18** A rule that needs a host or a secret is a definition whose own allow lines name them, and it has those and nothing more; such a rule may call a service and decide by a threshold on its answer.
*Accept:* `no-account-numbers.yaml`, its allow lines and the call naming a stand-in screening service, runs against it. Told to answer 0.01 for `leaks`, the rule says yes and the answer leaves; told 0.9, it refers with its view and Lee and Priya are asked, shown that note. The stand-in received the answer's text as `text` and the key as a bearer token.

**R-decisions-19** A rule that fails, does not end within 15 minutes of its start, cannot be started, or gives back no `says` Bulldog knows is taken as having referred, and the reason is its note in the thread; `pass`, which rules answered before `refer`, is a `says` Bulldog does not know.
*Accept:* five lists, each a rule then Sam: a rule that throws, one still running when the test's clock moves 15 minutes on, one whose file was removed after the office was read, one that returns `{ ok: true }` and one that returns `{ says: "pass" }`. Each decision goes on to Sam, and each thread holds a referral by the rule whose note says which of the five happened, the last two that its answer was not a decision, which is `{ says: yes, refuse or refer, note }`.

## Workers

**R-decisions-20** A worker named in a list is started from its own definition, as written in its own team's folder, a model named by one word filled from that team outward as for any worker, with the decision said to it, as a run of the team whose decision it is (C-decisions-23); its result `{ says, note }` is its answer, and a result in any other shape is a referral, noted in the thread.
*Accept:* the reviewer, a worker of the house whose reply has the shape `{ says, note }`, is started with the decision said to it and labelled as asked to decide it; it ends `{ says: "refuse", note: "That circuit feeds the freezer." }`, and the decision is refused with that note. A worker whose reply is free text is read as a referral, and the thread says its answer was not a decision. Payments written `disclosure: [ops, lee, priya]`, with platform's ops worker first: ops is started from platform's `ops.yaml`, and its run is labelled with payments; written `model: usual`, it runs on the `usual` platform's file or a team around platform gives, never one payments gives.

**R-decisions-21** A rule or a worker asked to decide is told that its yes or its refusal ends the decision, and where its referral would go.
*Accept:* with `cut_power: [reviewer, sam]` the reviewer's input names Sam in `referral_goes_to`, as who a referral goes to. With `cut_power: [reviewer]` it names the house's `everything`, Sam. For the garden's `everything: [./a.js]`, the rule's input names the house's `everything` as where a referral goes.

## What a decider is shown

**R-decisions-22** A decider of a held action is shown the team, the worker asking and what it said with the call, the action as its tool describes itself, the arguments it would be given, the request it serves and who asked it, who has referred it before it with their notes, and where its own referral would go, or for a person who else is asked.
*Accept:* Lee's direct message and the decision on Lee's screens for the refund each hold: payments, the ledger, the ledger's reasoning, "Refund one charge", the charge's arguments, the request the refund serves, payments' sub-request "was 4411 charged twice?" asked by the bot, of the office, and Priya named as asked too; nothing of Dana's request above it, which is the office's (C-decisions-19). Asked about a £300 `order_part`, Sam is shown "small-orders.js referred it: £300 is more than I order alone.".

**R-decisions-23** A decider of a disclosure is shown what would leave and whether it is an answer, a question, a tell or a sub-request; for an answer, the question it answers, and for the others, the request it is said in and who it is to; where it will be read; the team's notes; and who has referred it before it, with their notes. What it decides is named in a line built as R-decisions-31 says, a sub-request naming the team asked.
*Accept:* `no-account-numbers.yaml` is given the ledger's answer, the bot's question, `#help` as where it will be read, and the text of payments' notes. With #help listed in the Slack bridge's settings as a place where payments is asked, Dana's request there goes to payments, and for the ledger's tell to Dana, "looking at the charges now", it is given the tell, Dana's request, Dana as who it is to, #help as where it will be read, and the same notes. For the ledger's question to platform, it is given the question, payments' sub-request, platform as who it is to, and the same notes, and it decides "Question to Platform, to disclose". Lee, asked after the rule refers it, is shown the rule's referral and note.

**R-decisions-24** A decider of a change is shown who drafted it, why, what it changes and what it lets more by, as the change is described when it is put, and who has referred it before it, with their notes.
*Accept:* the decision of a change that adds a tool to the fixer shows the builder, its reason, the files changed and the sentence saying the fixer may now do more.

## Reaching people

**R-decisions-25** A person who is asked sees the decision on the screens, waiting on them, and is sent a direct message on each bridge they are reached on, by that bridge's out machine, holding what they are shown and how to answer.
*Accept:* in the house as the office page shows it, with its Telegram bridge, Sam, reached on Telegram, is asked about `cut_power`: the Telegram bridge's out machine is started once with a direct message to Sam's Telegram identity, and the decision is in what waits on Sam on the screens. A person reached on no bridge is asked on the screens alone. With the Telegram bridge stopped by a problem of the reading, Sam is asked on the screens, no out machine of it is started, and the thread says the direct message could not be sent on Telegram.

**R-decisions-26** An answer brought by an in machine is taken only when the identity the bridge checked belongs to a person who may decide that decision.
*Accept:* Lee's Slack identity says yes to the refund and the yes is taken. The same answer under Raj's Slack identity, whom no list for payments names, is not taken, and Raj is told he does not decide it. Under an identity that matches no person, it is not taken.

**R-decisions-27** When a decision ends, everyone else who was asked is told it is decided, by whom, where they were asked.
*Accept:* after Lee's yes, Priya's direct message on Slack is followed by one saying Lee said yes at 14:02, and the decision is no longer in what waits on Priya on the screens.

## Final

**R-decisions-28** A decision is final: the first yes or refusal taken stands, and any later answer changes nothing and tells whoever gave it who decided and when.
*Accept:* Sam refuses on the screens; Sam's yes a second later on Telegram is not taken, and the out machine tells Sam it was refused by Sam at that time.

**R-decisions-29** Each decider's answer, and each referral, is an entry in the request's thread, by the decider and to the worker whose decision it is; a disclosure's entries are to the team's lead, and a change's to whoever drafted it.
*Accept:* the refund's thread holds the ledger's call, then "approved by Lee" by Lee to the ledger. The disclosure's thread holds the rule's yes, by `no-account-numbers.yaml`, to the ledger.

**R-decisions-30** While a decision is asked of people, the request it serves is waiting on them; while it is with a rule or a worker, it is working.
*Accept:* while Lee and Priya are asked, the office's request shows waiting on Lee and Priya; while `small-orders.js` runs, the house's request shows working.

**R-decisions-31** What was done holds each decision from the moment it is asked: what it is, who it waits on, and once it ends, how it ended, who decided and when; what it is is built from the decision and the office as read each time it is written or shown, and is not kept with it: a held action's description, what would be disclosed, a question to another team naming the team asked, or the change by the first line of its why; its arguments, the worker's reasoning and every note are what was read, of the team whose decision it is.
*Accept:* while Lee and Priya are asked, Dana's screens show "Refund one charge: waiting on Lee, Priya" on her request; afterwards "Refund one charge: approved by Lee, 14:02", and never the charge's arguments, the ledger's reasoning or Lee's note. Lee's show all of them.

## Where the answer goes

**R-decisions-32** A disclosed answer goes on to whoever asked exactly as it was decided. A refused answer does not leave: it goes back as an entry by the decider holding the note, always to the team's lead, which starts the lead again on the same request within work's limit on runs in a row, or asks it again where the lead is a person. While the lead tries again, whoever asked the team hears only what it says next and is disclosed, or how its run ended, as work says.
*Accept:* `no-account-numbers.yaml` says yes and the bot receives "Duplicate confirmed; £40 refunded." unchanged. Lee refuses a disclosure with "that names the card": the bot's tool still waits and is given nothing; the ledger is started again on payments' sub-request, given Lee's entry with the note, and its next answer, which the rule lets go, is what the bot receives. Support, whose lead is Dana, with a disclosure rule that refuses her answer: the sub-request waits on Dana again, with the rule's entry to her.

**R-decisions-40** Whoever asked a team is told of a refused disclosure in one case only: when a refused answer has used up work's limit on runs in a row, or the lead, started again after a refusal, gives up by ending with no answer. Bulldog then answers them with one fixed line, "<team> did not disclose its answer", naming the decider of the last refusal, and with nothing of the answer or of any note; that line stands in place of the team having given no answer.
*Accept:* with a disclosure rule `refuse-all.js` that refuses every answer, the ledger is started again until work's limit is reached; the bot then receives "payments did not disclose its answer (refuse-all.js)", and none of the answers nor any note appears anywhere outside payments. With Lee refusing the first answer and the ledger, started again, ending with an empty answer, the bot receives "payments did not disclose its answer (Lee)" and not that payments gave no answer.

**R-decisions-37** A refused question, tell or sub-request does not leave the team. Its refusal is an entry by the decider to the team's lead, holding the note, and is given as the tool's result to the run that said it while that tool waits, and otherwise starts the lead again as a refused answer does. Nobody outside the team is told anything.
*Accept:* the ledger asks platform about a charge with a card number in the question. The rule refers it, and Lee refuses with "no card numbers to platform". No sub-request to platform is ever made; the ledger's tool result says the question was not disclosed, by Lee, with the note, and its run goes on. Al, Dana and platform see nothing of it.

**R-decisions-33** A change approved by a person is handed to be written, with who decided and when; a refused one is not written, and whoever drafted it is told, with the note.
*Accept:* Al says yes to a change; the commit that writes it names Al as having decided. Al refuses another with "not the fixer's job"; nothing is written and the builder is told it was refused by Al with that note.

## Withdrawn, re-read and restarted

**R-decisions-34** A decision whose run ends while it waits, by cancelling, failing or the ask's own timeout, is withdrawn: any rule or worker running on it is cancelled, everyone asked is told, and a later answer to it changes nothing.
*Accept:* the fixer's run is cancelled while Sam is asked about `cut_power`. The decision shows withdrawn, Sam's Telegram is told it is no longer asked, and Sam's yes after that is refused as withdrawn. A rule running on another held job of that run is cancelled. The test's clock moves 7 days on while Sam is asked about another `cut_power`: the ask ends, Corbie denies the job, and that decision shows withdrawn too.

**R-decisions-35** A decision outlives a restart of Bulldog: the walk goes on where it was, a rule or worker that ended while Bulldog was away is read when it is back, and a person's answer is taken as before.
*Accept:* Bulldog stops while Lee and Priya are asked, and again while a rule runs. Started again, Lee's yes is taken and the refund runs once; the rule that ended while Bulldog was away has its answer read, and the walk goes on from it, with no decider asked twice.

**R-decisions-36** When a reading of the office takes a decider a decision waits on off its list, or out of the office, the decision walks on at that reading: whoever is no longer on the list is told they are no longer asked and a later answer from them is not taken, a rule or worker no longer on it is cancelled, and the walk goes on from the step it had reached over the list as now read; a disclosure whose team no longer sets `disclosure` is let go at that reading; and a decision that the lists as now read let nobody decide is refused (R-decisions-38).
*Accept:* Lee and Priya are asked about the refund, and payments' file is rewritten `decide: { everything: [priya] }`. Once it is read, the refund waits on Priya alone, Lee's Slack is told he is no longer asked, and his yes after that is not taken. Rewritten instead `decide: { everything: [raj] }`, Raj is asked, and Lee and Priya are told they are not. With an answer of payments held for disclosure, payments' file rewritten without `disclosure` lets the answer go on to the bot once it is read.

## Constraints

**C-decisions-01** A person's answer is taken only from their checked identity: on the screens the person signed in, and from a bridge the identity its in machine checked. What a message says about who sent it is not an identity. *Test:* `decisions_answer_only_from_checked_identity`

**C-decisions-02** An answer is matched to its decision by the decision it names, and to its decider by who gave it, never by the conversation it came from. Matching by conversation drops answers given in a group and takes answers from whoever shares a chat. *Test:* `decisions_answer_matched_by_decision_and_identity`

**C-decisions-03** A guest never decides, and a list never names one. A guest is someone the office knows nothing about. *Test:* `decisions_guest_never_decides`

**C-decisions-04** A worker never decides its own held action: reaching it in the walk counts as its referral. An action's own author cannot be the check on it. *Test:* `decisions_worker_never_decides_own_action`

**C-decisions-05** Corbie gives a held job to the nearest ask at or above its machine, so a worker Bulldog starts at the top, a team's lead or a routine's worker, names the ask Bulldog ships if anything it runs can be held; a colleague called as a tool uses the nearest ask above it unless it names the ask Bulldog ships itself, which it may, and must if it can also be started at the top. Every ask a worker names is Bulldog's, and naming any other is a problem (R-office-30). A held action is the action of the worker whose run holds it, found by walking up the run's parents to the nearest worker, and is decided by that worker's team, whichever ask it reached; its actions are its own, not its caller's. Corbie refuses a machine whose held jobs have no ask to go to, and the ask a held job reaches says nothing of whose action it is. *Test:* `decisions_held_action_belongs_to_nearest_worker`

**C-decisions-08** A decision is written before Corbie's ticket is answered or an answer is handed on, and the answer given to a ticket is kept, so one found waiting again after a restart is given the same answer. A decision that cannot be written leaves the action held, and the person who gave it is told it was not taken. Otherwise a restart does an action twice, or an action is decided and never answered. *Test:* `decisions_written_before_answered`

**C-decisions-09** Two answers that arrive together are taken one at a time, and only the first changes the decision. Two people tapping yes and no at once must not both be taken. *Test:* `decisions_first_of_two_answers_stands`

**C-decisions-10** A person's answer cancels any rule or worker run asked of the same decision, and that run's result is not read. A rule must not overturn a person, nor go on spending after it is not needed. *Test:* `decisions_person_cancels_rule_in_flight`

**C-decisions-11** A rule that has not ended 15 minutes after its start, and a worker asked to decide that has not ended an hour after its start, are cancelled and refer, as does one that fails, cannot be started, or gives back no answer Bulldog knows, with the reason as its note. A rule judges data it is given whole, and a service it calls answers in seconds; a worker may wait its turn behind the office's `runs:`. Both limits sit far inside the 7 days the ask Bulldog ships waits, and when the ask ends first its deciders are cancelled with the decision (R-decisions-34), so no decider outlasts the ask it decides for. A broken decider must never leave an action waiting for ever. *Test:* `decisions_failed_decider_refers`

**C-decisions-12** No decision waits on nobody. The only check a reading makes that a walk can reach anyone is that the office's `everything` reaches a person (C-office-06), so a walk that is not ended on the way comes to a person; a reading that takes away the deciders a decision waits on walks it on (R-decisions-36); and a decision that nobody can be reached to decide is refused, saying why (R-decisions-38). A decision waiting on nobody holds its action for ever with no one told. *Test:* `decisions_every_walk_ends_at_a_person`

**C-decisions-13** Every person asked is asked on the screens, whatever their bridges; a direct message that cannot be sent leaves them asked there, and the failure is written in the request's thread. A decision must never wait where nothing reaches the person. *Test:* `decisions_person_always_asked_on_screens`

**C-decisions-14** What a rule or worker is told about where its referral goes is what the walk will do. A decider that refers so that a person may look, told of a person who will never see it, decides wrongly. *Test:* `decisions_decider_told_truth_about_after`

**C-decisions-17** Each held job is decided by itself, and a rule or worker is given exactly one decision. A batch would make one decider's referral on one job a referral on all. *Test:* `decisions_one_job_one_decision`

**C-decisions-18** Bulldog grants a rule no permission and writes no definition around one, a bare script being wrapped by Corbie's client in a flow of one stage: a bare script runs with none, and a rule that is a definition with what its own allow lines give. A bare script that reaches for a host or a secret is refused by Corbie, and refers with that as its note. What a rule may reach is read in its own file, as a worker's is, and a grant from Bulldog would be one no person wrote. *Test:* `decisions_rule_gets_no_grant`

**C-decisions-19** A decider outside the team, a person further out or a worker of another team, is given the one decision it is asked and nothing else the team read. Teams are sealed, and being asked one question opens nothing else. *Test:* `decisions_outside_decider_sees_only_the_decision`

**C-decisions-20** A decision's details go only to the people asked, by direct message, and never to a channel or a group. A channel's audience is wider than the deciders and grows after a message is posted. *Test:* `decisions_details_only_by_direct_message`

**C-decisions-21** What a team discloses is exactly the text decided, and a refused disclosure lets out nothing of what was held or of any decider's note. A note that explains a refusal may repeat what was refused. *Test:* `decisions_disclosure_lets_out_exactly_what_was_decided`

**C-decisions-22** A team's notes are read afresh for each disclosure decision. A check that reads an old copy lets out what the team has since said it never lets out. *Test:* `decisions_disclosure_reads_current_notes`

**C-decisions-23** A worker asked to decide is started from its own definition, with what its own allow lines give, and its run belongs to the team whose decision it is, even when the worker is of another team: it carries that team's label with the labels of the decision it is asked, and what it read is seen only by that team's people. It reads that team's decision, and what a team's decision holds is that team's; being asked opens nothing of the worker's own team to the team it decides for, nor of that team to the worker's own. *Test:* `decisions_worker_decider_run_belongs_to_team_decided_for`

**C-decisions-24** Every job held from one ask is written as a decision before any of them is asked of a decider. A rule's yes could otherwise decide the first at once and answer the ask's ticket before the others exist, and they would be denied without anyone being asked. *Test:* `decisions_all_held_jobs_written_before_any_asked`

**C-decisions-25** A rule's or worker's result is taken only from the run the decision is waiting on at that step, and only once. A result seen by the watch and again on settling, or one from a run the decision no longer waits on, changes nothing. Taken twice, one referral would skip the next decider; taken late, it would move a decision already past it. *Test:* `decisions_decider_result_taken_once_from_its_run`

**C-decisions-26** A held job that no model called, a flow's step or a script, is shown with its input as the arguments it would be given. Corbie gives a call's arguments only for a model's tools, and a job shown with nothing is decided blind; a run's input as typed data, given by Corbie, would make this one path. *Test:* `decisions_held_job_without_a_call_shows_its_input`

**C-decisions-27** A refusal of a disclosure is told to nobody outside the team: whoever asked hears of it only by the one fixed line of R-decisions-40, when the limit on runs in a row is used up or the lead gives up, and otherwise only what the lead later says and is disclosed, or how its run ended. A refusal said outside tells the asker something was held back while the lead may yet answer well. *Test:* `decisions_refused_disclosure_not_told_outside`

## Interfaces

- **Office** reads each team's `decide` and `disclosure` lists, and the shape of their keys, and reports a rule or worker after a person (C-office-20) and a decider that is none of a person, a worker, a rule in the team's own folder or a rule Bulldog ships (C-office-21) as problems, with every other problem; it makes the office's `everything` reach a person (C-office-06), the one check of deciders a reading makes, on which C-decisions-12 rests. It reports, as that worker's problem, a worker started at the top that can hold a job and names no ask, and any worker that names an ask other than Bulldog's (R-office-30), on which C-decisions-05 rests, and fills a model named by one word from the worker's own team outward, as R-decisions-20 starts a deciding worker. It gives the people a team's decisions reach (R-decisions-39), which Requests and this area both take from it, and reports each line of a rule's files that answers `pass` or reads `passes`, without stopping the rule (R-office-38), beside R-decisions-19.
- **Requests** hold each decision's entries in the request's thread, show the request as waiting on the people asked, and show what was done and what was read to whom they belong; they make the people a team's decisions reach (R-decisions-39) the people of a team with none, for every purpose of sight and control (R-requests-36, C-requests-15); and they hand here the answer of a lead who is a person that a disclosure check holds (R-decisions-04), as soon as it is kept, whether it was said on the screens or through a bridge; one held when Bulldog stopped before handing it is handed, once, as this area reads the office.
- **Work** finds the ask's runs by their labels; hands every worker's answer, question, tell and sub-request leaving a team that set a disclosure check here (R-work-27), the answer of a lead who is a person being handed by Requests; starts a refused answer's lead again within its limit on runs in a row (R-work-46, C-work-04), and says the fixed line of R-decisions-40 when the limit is used up or the lead gives up; starts, labels and cancels the runs of rules and of workers asked to decide, a worker's with the team C-decisions-23 names (R-work-47); and, holding the clock that routines and timers use, calls `Due` here each time it moves, so a rule or a worker asked to decide that has run past its limit refers (C-decisions-11, R-decisions-19); a call with nothing due changes nothing.
- **Bridges** carry a decision's direct message out through an out machine, and bring an answer back through an in machine with the decision it names and the sender's checked identity; whether that answer is taken is R-decisions-26 and R-decisions-28.
- **Changes** say which list decides a change (R-changes-17), who may put one and how a person's own change is walked, describe a change and what it lets more by when it is put, and write it once a person approves, naming who decided; that only a person's yes approves a change is C-changes-20, and that its deciders judge the office as it is is C-changes-21.
- **Server** shows a signed-in person the decisions waiting on them, each named as R-decisions-31 builds it, and takes their yes or refusal on the screens; it shows who decides a tool, and `check` lists it, by the walk this area gives for a team and a key (R-decisions-13, R-server-32, R-server-03).
