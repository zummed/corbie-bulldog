# Screens

The rules behind the [Screens overview](../screens.md). Area code: `screens`. A test serves the house or the work office over a whole Corbie in its own process, and asks as a browser would.

## Signing in

**R-screens-01** A person's key is the office secret's signature over their id and a hash of their entry in `everyone`, and `corbie bulldog key <directory> <person>` prints a key link, whose page's button signs them in.
*Accept:* `corbie bulldog key house sam` prints a link. Opening it shows a page with a button, and pressing it gives the browser Sam's key; Sam's Requests screen then loads.

**C-screens-01** A key ends with the entry it was signed over: changing or removing a person's entry signs them out, and replacing the office's secret signs everyone out. A key that outlived its entry would sign in a stranger written later under the same name, and a lost browser could never be signed out. *Test:* `screens_key_ends_with_entry`

**R-screens-02** Given a name, the sign-in page sets a random cookie in the browser and answers the same whatever the name, and sends a link signed over that cookie, good for 15 minutes, in the person's direct messages on the first bridge in `bridges:` that can message them directly; the link signs in only in that browser.
*Accept:* Sam asks on the sign-in page, and the link arrives in Sam's Telegram chat. Pressed in that browser it signs Sam in; pressed in another browser, or in Kit's, or 16 minutes later, it signs nobody in. The page's answer to `sam` and to `nobody` is the same.

**C-screens-02** A link signs in only when the button on its page is pressed, a POST, never when the link is opened. A service that carries a link opens it to show a preview. *Test:* `screens_link_survives_preview`

**C-screens-03** At most one sign-in link in each minute is sent to a person, and the sign-in page answers the same, and at once, whatever the name. The page is open to anyone, who could otherwise fill a person's messages, or learn who is in the office. *Test:* `screens_sign_in_links_limited`

**C-screens-04** The key is kept only in an HttpOnly, SameSite=Strict cookie, Secure when the screens are reached at an `https://` address. A key a page's script can read can be taken by a script, a cookie sent with a request from another site lets that site act as the person, and one sent over plain http can be read on its way. *Test:* `screens_key_only_in_cookie`

**C-screens-05** A key is compared in constant time. A comparison that stops at the first difference tells how near a guess came. *Test:* none: a test cannot time a comparison finely enough to prove it, so it is kept by review.

**C-screens-11** A POST or a PUT that a page of another site sends is refused 403. Another site's page could otherwise act as the person signed in, or sign the browser in as someone else and read what they then ask. *Test:* `screens_other_sites_refused`

## Who sees what

**R-screens-03** A person of a request's team sees it whole, and a team with no people has the office's people as its people; the person who asked it, the people of the team that asked a sub-request, and the people a question or a decision of it waits on see it sealed.
*Accept:* in the work office, Dana sees her request of payments sealed and no other request of payments. Lee and Priya see every request of payments whole, and Al sees every request of platform whole. Al sees payments' request that the bot asked, sealed, and Dana does not.

**C-screens-06** To someone not of the team, a request shows only its source, what was said there, whom it waits on, the outcome as sent, and each fork as who, when and how it ended; questions, answers, notes, held jobs, the lead's conversation and every stage's output are shown only to the team's people, and a question or a decision to the people it is put to; a question Bulldog refused is put to no one. The check exists to hold back what the team read, and a question to someone the team may not ask can carry it. *Test:* `screens_outsider_sees_nothing_the_team_read`

**R-screens-04** A question or a decision is shown, with its notes and the held jobs, to the people it is put to.
*Accept:* Sam's card for the fixer's order shows the order with `pounds: 35`, and the notes of `small-orders` and the clerk, above `yes` and `no`.

**C-screens-07** A request, a doc or a file the one looking may not see is answered 404, with the same sentence as one that is not there. A refusal that differs tells them it exists. *Test:* `screens_unseen_is_not_there`

**C-screens-08** The stream names only the requests the one looking may see, with sight read when each change is sent. A change named to every screen would tell everyone of requests they may not see. *Test:* `screens_events_filtered_by_sight`

## The screens

**R-screens-05** The Requests screen holds, from the top, a box to ask a team, the questions and decisions waiting on the one looking as cards with their choices, and the requests they may see, newest first, each a row with its flow as dots on a line, its team, who asked, its first line and when.
*Accept:* signed in as Sam during Kit's request, the screen shows the order's card with `yes` and `no`, then Kit's request, with its WORK dot waiting.

**R-screens-06** A request is drawn as its team's room, with SOURCE, HEED, WORK, CHECK and REPLY on one line, forks dropping from WORK, and the thread below, each node and state as the overview's list says.
*Accept:* Dana's answered request in #refunds draws every node passed. A noted one ends the line at a dashed HEED. A withheld one draws CHECK hollow `--bad` with a dashed line to REPLY. A stopped one draws the stage that was running filled `--ink3`, with `stopped` under it. The bot's request draws a TEAM fork to payments, which links to payments' request for Al, of the office, and does not for Dana, who asked the bot.

**R-screens-07** A team's page draws its room with no run, each node showing what is written for it, and beside it the team's people, workers as sent to Corbie, routines with their next time, docs, problems, and request machine as sent to Corbie; the office's page draws the teams inside its room and each bridge's in run, running, failing with Corbie's error, or stopped.
*Accept:* payments' page shows #refunds and the screens at SOURCE, `mentioned` at HEED, the ledger at WORK, `no-account-numbers`, Lee and Priya at CHECK, and `decide` as a fork line. The work office's page shows Slack running; with the stand-in Slack refusing its token, it shows Slack failing with Corbie's error.

**R-screens-08** A signed-in person may ask any team on the screens, with files; the person who asked a request, and its team's people, may say more in its thread and stop it.
*Accept:* Dana asks payments with a screenshot. Payments' request is given the file as a blob, and Dana's next message from that request's screen is the next request of its thread. Lee may say more there; Al is answered as if it were not there.

**R-screens-09** A team's people read and edit its docs on its page, through a run of `./.bulldog/docs/docs.yaml` started with the team's start list.
*Accept:* Lee edits `refunds.md` on payments' page, and the ledger's next `read_doc` gives the new text. The docs run carries `kind=docs` and `team=payments`.

## The API

**R-screens-10** The API is the overview's first table, JSON in and out, and the requests of its second table answer pages; an answer of the API that is not a success is `{ error }`, with a sentence a person can read.
*Accept:* each request of the tables, made as Sam on the house, gives or does what its row says. A body that is not JSON is answered 400 with `error`.

**C-screens-09** A file is served with its type told from what it holds, never from its name, and with a sandbox that lets it run nothing; only a picture other than SVG, or a PDF, is shown, and anything else is downloaded. A file someone put in must never run as a page of the office, where the key's cookie is. *Test:* `screens_files_served_harmlessly`

**C-screens-10** The screens load nothing from any other site, and every page is served with a policy that allows nothing else. A model writes the answers and docs the screens show, and a page that loads from elsewhere hands that site the office's screens. *Test:* `screens_load_nothing_from_elsewhere`

## Interfaces

Uses:

- **Office.** The teams, people, problems and docs' place, and the office's secret, for keys, through the reading Requests holds.
- **Requests.** The reading of the office, and whether a team takes requests; the newest requests a person may see; each request in full, with its decisions and the calls each colleague made; the waits on a person; asking, saying more, answering a wait, and stopping; the files said with a request.
- **Bridges.** Through an interface the program fills with the office's bridges: carrying a sign-in link to a person directly, which bridges reach people directly, and how each bridge's in run stands, as the bridges' `State`.
- **Corbie.** Its client, to start a docs run and wait for its end, and to follow the office's runs for the stream of changes.

Used by:

- **The program.** `corbie bulldog <directory>` serves the screens, and `corbie bulldog key` prints a key link.
