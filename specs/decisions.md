# Decisions

A decision is a yes that work waits for. This area is what needs one, who is asked, in what order, what they are shown, how a person answers from wherever they are, and where the answer goes.

## What waits for a yes

Two things do.

- **A held action.** A worker's definition, or a tools file it names, marks a tool for approval, or lists a permission under `ask_about:`, and Corbie holds the job before it starts. Corbie gives a held job to the nearest ask at or above its machine, and refuses a machine whose held jobs have no ask to go to: a definition started at the top when it is checked, a child when it starts. So a worker Bulldog starts at the top, a team's lead or a routine's worker, names the ask Bulldog ships as its `ask:` if anything it runs can be held, as the steward does. A colleague called as a tool uses the nearest ask above it unless it names the ask Bulldog ships itself, which it may, and must if it can also be started at the top. Every ask a worker names is Bulldog's; naming any other is that worker's problem. In the house the fixer is only ever called by the steward as a tool, so it names no ask, and its held `cut_power` goes to the ask the steward names; sparks, which holds nothing and is only ever called, names none either. Bulldog writes no ask into a definition, and finds the ask's runs by their labels. Corbie starts that ask, a machine that issues a ticket with the held jobs and waits. Each held job becomes a decision of its own, of the team whose worker holds it, whichever ask it reached. When every job of one ask is decided, Bulldog answers the ticket once, naming the jobs that may run and joining the notes. Corbie runs those, and tells the model of each other that it was denied, with the note.
- **A disclosure.** A team may set a `disclosure` check. Then everything it says to anyone outside it is held by Bulldog until it is disclosed, and only then goes on: every answer it gives, every question and tell its workers say, that anyone outside it reads, and every sub-request it makes. What is said to the asker is read by them and, at a place on a bridge, by everyone the bridge reports reading there, someone the office cannot name counting as outside, or, where the bridge never said who reads it, by any guest at all at a place guests may ask in, but a direct place. So an answer to the team's own people in the chat goes on at once, and at a place guests read, or may read for all the bridge has said, every answer is held.

A write to the office's files is an action like any other. `office_put` and `write_doc` wait for a yes because the kits Bulldog ships mark their jobs `approve: true`, and `write_notes` does not because its kit marks nothing; an office that copies a kit and changes that is decided as its copy says. A held `office_put` or `write_doc` is a decision of the team whose folder holds what it writes, which the changes page names, and its files are fixed when it is held, so whoever decides says yes to exactly what is written. A person's own change from the screens is decided the same way, as if they had called `office_put`.

## Deciders

Each team writes who decides what in its team file. A list names deciders in order, and each is one of three things:

- **A rule**, in the team's folder or among those Bulldog ships in `.bulldog/rules/`, named by its path. It is given the decision as data and gives back `{ says: yes | refuse | refer, note }`. A rule that needs nothing is a bare script, and decides from its input alone. A rule that needs anything, a host, a secret, a service that answers a question about it, is a Corbie definition, which holds its own allow lines like any other. Bulldog grants a rule nothing and writes no definition around one; a bare script is wrapped by Corbie's client, in a flow of one stage that runs it with the decision as input and no permission. A team's heed, named the same way, is a rule too, but no decider: nothing waits on it, and the office page describes it.
- **A worker** of any team, named by its name. It is started from its own definition, as written in its own team's folder, a model named by one word filled from that team outward as for any worker, with the decision said to it, and ends with the same `says` and `note`. Its run belongs to the team whose decision it is: a worker of platform deciding for payments is labelled with payments, and what it read there is seen only by payments' people.
- **A person**, named by their name.

A rule or a worker may be marked on a list to **advise**, written as a mapping of its name or path to the word `advise`: `junos_change: [reviewer: advise, paul]`. One that advises may refuse, which ends the decision, or refer, as any other; but its yes does not approve. It goes on to the next decider exactly as a referral does, with its note, and is shown as a recommendation of yes: "reviewer recommends yes: only opens 443 to the proxy". So a check that looks first and a person who decides after it can be written. A person is never marked, since a person never refers. The same worker may advise on one list and decide plainly on another.

A list need not name a person: one of rules and workers alone is whole, and what they all refer goes on, as below. Where a list names people, they come last, after every rule and worker. A team moves an approval to full automation by taking the person off the end of its list, so that `refund: [refund-check, lee]` becomes `refund: [refund-check]`; that is itself a change to the files, decided like any other.

```yaml
# the house, with a reviewer added: reviewer.yaml, a worker in its folder
people: [sam, kit]
lead: steward
decide:
  everything: [sam]
  order_part: [./small-orders.js, sam]  # Sam is asked only when the rule refers
  cut_power:  [reviewer, sam]           # the reviewer decides, or refers to Sam with its view
  open_door:  [reviewer: advise, sam]   # the reviewer may refuse; its yes goes to Sam as a recommendation

# payments
people: [lee, priya]
lead: ledger
decide: { everything: [lee, priya] }
disclosure: [./no-account-numbers.yaml]    # what it refers goes to Lee or Priya
```

```js
// small-orders.js: needs nothing, so a bare script
const pounds = input.action.args.pounds
if (pounds <= 20) return { says: "yes", note: "Under £20." }
return { says: "refer", note: `£${pounds} is more than I order alone.` }
```

```yaml
# payments/no-account-numbers.yaml: needs the screening service, so a definition
flow: no-account-numbers
input: { text: string }
stages:
  check:
    job:
      script: ./no-account-numbers.js
      input: { text: "${input.text}" }
      allow: [net:screen.example.com, "secret:SCREEN_KEY@net:screen.example.com"]
output: ${stages.check.output}
```

```js
// no-account-numbers.js
const key = secrets.get("SCREEN_KEY")
const found = http.post("https://screen.example.com/v1/screen", {
  headers: { Authorization: secrets.text`Bearer ${key}` },
  json: { text: input.text, find: "an account or card number" }, as: "json",
}).body.likelihood
if (found < 0.05) return { says: "yes", note: "No account number." }
return { says: "refer", note: `It may hold an account number (${found.toFixed(2)}).` }
```

## Which list

A held action is decided by the team's list named for the job, such as `order_part` or `office_put`, or else by `everything`. A disclosure is decided by `disclosure`. A team that writes no `everything` has its people as that list.

A list in which every decider refers goes on to the team's `everything`, whatever list it was: a `disclosure` that refers goes to the team's `everything` as an `order_part` does. An `everything` in which every decider refers goes on outward, to the `everything` of the team directly around it, and so does one that names nobody because the team has no people; and so on out to the office. A decider outside the team, reached this way, is shown the one decision it is asked, and nothing else the team read.

The people a team's decisions reach are the people this walk comes to from its `everything` when every rule and worker on the way refers: those its own `everything` names, or, where it names none, those of the first `everything` outward that does. Platform has no people and writes no `decide`, so its decisions reach Al, through the office's `everything`. For a team with no people, they stand as its people, as the requests page says.

The only check a reading makes that a walk can reach anyone is that the office's `everything` reaches a person, so a walk that is not ended on the way comes to a person. A decision that nobody can be reached to decide, such as one whose team is no longer in the office, is refused by Bulldog, saying why. Nothing waits on nobody.

## The walk

Deciders are asked in order. A yes or a refusal from any of them ends the decision, but for the yes of one that advises, which goes on as a referral does, as a recommendation. A referral goes to the next, carrying its note, so a rule or a worker that wants a person to look refers with its view: "looks like a duplicate, 0.93". Whoever is asked after it is shown every referral and recommendation so far, with its note. A rule or worker is told that its yes or its refusal ends the decision, and where its referral would go; one that advises is told instead that its refusal ends it, and that its yes goes on as a recommendation to whoever its referral would go to. A recommendation from one that is last on its list goes wherever its referral would, to the team's `everything` and on outward. One that advises is held to what it was told: its yes goes on as a recommendation even if the lists are read again, or Bulldog restarts, while it is asked. A worker that advises on one list and decides plainly on the team's `everything` is asked once on a walk, as one that referred is.

People come last in a list. When the walk reaches the first person, everyone from there to the end of the list is asked at once, and the first to answer decides. A person says yes or refuses; they do not refer.

A worker never decides its own action.

A rule is given the referrals before it as `referrals`, each with its `says`, `refer` or, for a recommendation, `yes`; where its own would go as `referral_goes_to`; and whether it advises as `advises`. One that still answers `pass`, as rules did before `refer`, gives no answer Bulldog knows, which is a referral with that as its note, and the office reports each such line of its files as a problem.

The walk is the same whether a decision waits on it or not: given a team and the key of a list, this area gives the deciders it would ask, in order, as far as the people it ends at, and the screens and `check` show who decides a tool by it.

## A write

A held `office_put` or `write_doc` is walked, shown and answered here as any held action is, and how it was decided is handed to the changes area, so the write that runs after its yes names who decided and when. Whose decision it is, that it is judged by the deciders of the office as it is, who may put a change from the screens and how a person's own change is decided are the changes page's. One the changes area cannot put, a draft that is empty, says no why or has a problem, is refused by Bulldog as soon as it is held, with what the check says as its note, and nobody is asked.

## What a decider is shown

For a held action: the team, the worker asking and what it said about it, the action as its tool describes itself, the arguments it would be given, the request it serves and who asked it; and for a held write, as for a person's own change, who drafted it, why, what it changes, what it lets more by, and every file whole beside what it replaces, which a direct message gives by the lines it takes away and adds. For a disclosure: what is said, whether an answer, a question, a tell or a sub-request; who it is to, a team asked by its folder name beside what it is called, and where it will be read; the request it serves and, for an answer, the question it answers; and the team's notes, which may say in prose what the team never lets out. On the screens, a person asked a disclosure is shown its text, whom it is to, where it may in the end be read, which the team a sub-request asks is not bound by, and the requests it goes through from the top of the tree, each by its team and number, ending with the team a sub-request asks, as "Desk #12 › Insights #13 › Payments, new" until it is made and by its number after; and this only to those asked it. Every decider is also shown who has referred it or recommended yes before it, with their notes, and where a referral of its own would go. A person asked sees each recommendation in plain words, "reviewer recommends yes: only opens 443 to the proxy", above their Approve and Refuse, on the screens and in the direct message a bridge carries.

## Reaching people

A person who is asked sees the decision on the screens, waiting on them, and is sent a direct message on each bridge they are reached on: Bulldog starts that bridge's out machine with the decision and how to answer. A bridge stopped by a problem sends nothing, and the person is asked on the screens and their other bridges. Where a person is reached is their entry in the office files, so changing it, their own entry included, is a change decided like any other: a yes is taken from an identity only because the files, as decided, name it. An answer that comes back through an in machine is taken only when the identity the bridge checked is that of a person who may decide it. A guest never decides.

Anyone the walk would reach may decide while the decision waits, before it reaches them. A person's answer cancels any rule or worker still running on it.

## Final

The first decision stands, a refusal as much as a yes. Each answer, each referral and each recommendation is an entry in the request's thread, by the decider, a recommendation saying it in plain words, "reviewer recommends yes", and never that the decider approved: to the worker whose action it is, and to the team's lead for a disclosure. A later answer changes nothing, and whoever gave it is told who decided and when. Everyone else who was asked is told it is decided, where they were asked.

What was done holds the decision from the moment it is asked: what it is, who it waits on, and then who approved or refused it, and when: "Refund one charge: approved by Lee, 14:02". What it is is built from the decision and the office as read each time it is written or shown, and is not kept with it: the action as its tool describes itself, a held write by the first line of why it was drafted; or what would be disclosed, by whom, to whom and, for an answer, where it is read, as "ledger's answer to Dana, in the chat on the screens" or "bot's request to Payments". The arguments, the worker's reasoning, every note and the walk are what was read, inside the team whose decision it is, and are shown to its people and to those asked it. A disclosure is what was read of the team until it is let go, so it is shown to the team's people and those asked it alone; a held action is shown, as what it is, how it stands and who ended it, to whoever sees what was done.

What is disclosed leaves exactly as it was decided. What is refused does not leave at all, and nothing of it or of the note goes outside the team. It goes back to the team's lead instead, always, whoever in the team said it, as an entry by the decider holding the note. A run still waiting on what it said, a question, a tell or a sub-request, is given the refusal as its tool's result and goes on. A refused answer, whose run has ended, starts the lead again on the same request, within work's limit on runs in a row; a lead that is a person is asked again.

Whoever asked the team hears nothing of a refusal while the lead tries again: they hear what the lead says next and is disclosed, or how its run ended, as work says. They are told of it in one case only: when a refused answer has used up the limit on runs in a row, or the lead, started again after a refusal, gives up by ending with no answer. Then Bulldog answers them with one fixed line, "<team> did not disclose its answer", naming the decider of the last refusal, such as "payments did not disclose its answer (Lee)", and nothing of the answer or of any note.

If the run that holds an action ends while the decision waits, the decision is withdrawn: what is running on it is cancelled and those asked are told. A decision outlives a restart of Bulldog, because the held job waits in Corbie and the walk is kept.

## Interfaces

- **Office** reads `decide` and `disclosure` from each team file, with the shape of their keys and which rules and workers are marked to advise, and reports a list that names an unknown decider, or a rule or worker after a person, as a problem; it refuses a reading whose office `everything` reaches no person. It reports, as that worker's problem, a worker started at the top that can hold a job and names no ask, and any worker that names an ask other than the one Bulldog ships, and it fills a model named by one word from the worker's own team outward. It gives the people a team's decisions reach, as this page walks them, which Requests and this area both take from it, and reports each line of a rule's files that answers `pass` or reads `passes`, naming the word that took its place, without stopping the rule.
- **Requests** keep each decision's entries in the request's thread, show a request as waiting on the people a decision is asked of, and apply who sees what was done and what was read, as this page says a decision shows in each; they make the people a team's decisions reach the people of a team with none, for every purpose of sight and control; and they hand here the answer of a lead who is a person, which a disclosure check holds, as soon as it is kept, whether it was said on the screens or through a bridge, so that no held answer waits on nothing; one held when Bulldog stopped before handing it is handed here, once, as this area reads the office.
- **Work** finds the ask's runs by their labels, hands every worker's answer, question, tell and sub-request leaving a team that set a disclosure check here (the answer of a lead who is a person is handed by Requests), starts a refused answer's lead again within its limit on runs in a row and says the fixed line when that limit is used up, and starts and labels the runs of rules and of workers asked to decide, each with the team this page says it belongs to. It holds the clock that routines and timers use, and calls `Due` here each time that clock moves, so a rule or a worker asked to decide that has run past its limit refers; a call with nothing due changes nothing.
- **Bridges** carry the direct message out and bring the answer back with the sender's checked identity.
- **Changes** fix a held `office_put` or `write_doc` when it is held, or say why it cannot be put, and name the team whose decision it is; say who may put a change from the screens and how a person's own change is decided; describe a change and what it lets more by; and are told how each write was decided, writing a person's own change on its yes and a held write when its job runs after it, recording who decided.
- **Server** shows a person what waits on them and takes their answer on the screens, each decision's line built as this page says, and shows who decides a tool, a list and a disclosure, and `check` lists who decides a tool, by the walk this area gives for a team and a key, or for a team's disclosure.
