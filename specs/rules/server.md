# Server: rules

The detailed rules behind the [Server overview](../server.md). Requirements are `R-server-nn`, constraints `C-server-nn`. "The house" and "the work office" are the two offices the office page shows as files; the house there is the fuller example, with its Telegram bridge, and not the office the server writes into an empty folder, which is the house without the bridge (R-server-06). A test writes one into a folder of its own and serves it on a whole Corbie in the test's process, with a stand-in model and stand-in bridges, and calls the server as a browser would. "A team's people", for a team with no people, are the people its decisions reach (R-requests-36). A test moves a clock it controls, and steps the work until it is still; it never waits a fixed time.

## The program

**R-server-01** `corbie bulldog` with a folder or none serves the office there; `corbie bulldog check` reads one and `corbie bulldog init` readies one, each without serving it.
*Accept:* `corbie bulldog --help` prints, on standard output, help that ends with `check`, `init`, `manual` and `mcp`. `corbie bulldog init <empty folder>` writes the example and ends with status 0, listening on nothing.

**R-server-02** The command line is read by Corbie's own program for one: `--help` and `-h` print the help on standard output; a mistake in how the line is written prints `corbie bulldog: …`, or `corbie bulldog check: …`, with the usage line, and ends with status 2; a failure prints the same prefix and ends with status 1; no command of Bulldog's own but `version` prints JSON, and `--json` is a mistake.
*Accept:* `corbie bulldog check a b` ends with status 2 and the usage line; `corbie bulldog --json` ends with status 2; `corbie bulldog check <folder with a problem>` ends with status 1, its output beginning `corbie bulldog check:`.

**C-server-01** `check` and `init` are commands before they are folders, each takes one folder or none and neither `--config` nor `--store`, and `./check` names a folder. A mistyped word or `help` taken for a folder would be made into an office. *Test:* `server_words_are_never_folders`

**R-server-03** `check` reads the folder as the server would, with no Corbie running, nothing served, nothing started and nothing written, and lists each team with its `about`, its people, its lead, its deciders, its disclosure check and its routines; each worker with its type, its model as it is sent, a model named by one word filled in from the nearest team that gives it, and its tools, and for each tool that waits for a yes, who decides it; each bridge; how many runs the office has at work at once; and then every problem.
*Accept:* `corbie bulldog check` of the work office lists four teams, payments with its `about`, `lee` and `priya`, `ledger` as its lead and its disclosure check of `./no-account-numbers.yaml`; the ledger with its refund tool marked as waiting for a yes, decided by `lee`, then `priya`; platform with `al` deciding for it, from the office's `everything`; two bridges; 3 runs at once; and no problem. The folder's files and their times are the same afterwards. For this check, `office.yaml` gives `models` a name `usual`, `payments/team.yaml` gives `usual` another model, and the ledger and the bot each write `model: usual`: the ledger is listed with payments' model and the bot with the office's. The ledger written with `model: quick`, a name no team gives, is listed as the ledger's problem, and every other worker reads.

**C-server-02** `check` reports every problem a reading of the office has, each in the words the reading gives it, with its file: a worker's model named by a word no team gives, a rule named as a decider, a bridge's entry, folder and machines, and a routine, as much as a worker Corbie refuses. A check that left out a broken rule would pass an office that the server then reports as broken, and one that labelled every team's problem as one kind would send a person to the wrong file. *Test:* `server_check_reports_every_problem`

**R-server-58** `corbie bulldog version` prints which Bulldog this is, the Corbie it is built on, and the machine types and plugins that Corbie holds, and with `--json` prints the same as JSON, as `corbie version` does. A release build is given its version; any other reads it from the module's build information.
*Accept:* `corbie bulldog version` prints `corbie bulldog `, its version, `built on corbie ` and Corbie's, and lists `agent` among the machine types and `http` among the plugins; `version --json` gives the same four.

**R-server-57** `corbie bulldog manual` and `corbie bulldog mcp` are Corbie's, given to the program. The manual holds Bulldog's pages first, `office`, one for each thing Bulldog ships, named by its path and holding its `README.md`, and `bulldog`, then Corbie's and a page for each command; a model connected to `mcp` is told the page `bulldog`, reads the manual, and runs only the commands it is allowed, under the tool `bulldog`.
*Accept:* `corbie bulldog manual` lists `office`, `tools/talk`, `bridges/slack`, `bulldog`, `corbie bulldog check` and Corbie's `agent`; `manual office` prints the page with the weather worker, and `manual tools/talk` the talk tools' page with `tools_file: [./.bulldog/tools/talk/talk.yaml]`. `corbie bulldog mcp --allow check`, connected, tells the model the page `bulldog` and offers the tools `manual` and `bulldog`; `check` of the house through `bulldog` ends 0 and lists the house; `init` through it is refused and makes no folder.

**R-server-04** `init` readies a folder as the server would, without serving it, and says what it did; with nothing to do, it says so.
*Accept:* `corbie bulldog init <empty folder>` says it wrote the example and made a repository; run again, it says there was nothing to do.

## Choosing and readying the folder

**R-server-05** With no folder named, the folder is the one the program is run in when it holds an `office.yaml`, and otherwise `bulldog` in the person's home.
*Accept:* run in a copy of the house with no folder named, it serves that copy. Run in an empty folder with no folder named, it serves `~/bulldog`, with the home set to a folder of the test's.

**R-server-06** A folder that has nothing in it, or `~/bulldog` when it is not there, is given the example office, the house the office page shows without its bridge, spoken to in the built-in chat alone, and made a git repository of its own; and the server says both.
*Accept:* serving a new empty folder writes the house there, with no `bridges/` folder, no `bridges` key and no Telegram identity under `everyone`, makes it a repository whose one commit holds the house's files, and prints both lines. The same with `~/bulldog` not there.

**C-server-03** A folder named that is not there is refused, naming `corbie bulldog init`, and nothing is made; only `~/bulldog`, when no folder is named, is made. A typo made into a folder is an office nobody meant. *Test:* `server_missing_folder_refused`

**C-server-04** A folder that has things in it and no `office.yaml` is refused and left as it was: nothing is written into it and it is not made a repository. Pointing the server at the wrong folder must not litter it. *Test:* `server_folder_that_is_no_office_untouched`

**C-server-05** A folder given the example is made a repository of its own even when it lies within another repository. Otherwise the office's changes would be committed into someone else's project. *Test:* `server_new_office_has_own_repository`

**R-server-07** An office in no repository is made one: what is there is committed by Bulldog as the office's first commit, and `.bulldog/` is left out of it.
*Accept:* a copy of the house with no `.git/` is served; the repository has one commit holding every file of the house and none under `.bulldog/`, and a file added under `.bulldog/` later is not shown as an edit by hand.

**C-server-06** The example written into a new office, the house without its bridge, reads with no problem, and a person can ask it something in the chat as soon as it is served. A person's first sight of Bulldog must not be a list of problems. *Test:* `server_example_reads_well`

**R-server-08** When there is nothing to make, the server says only what it serves.
*Accept:* serving the house a second time prints which Corbie, the office's line, the listening line and the links, and no line about writing or committing.

## Which Corbie

**R-server-09** Bulldog is a client of the Corbie set up on this computer, and runs no role; `corbie start controller runner bulldog` runs it beside the controller and the runner, and when one of the three stops, all stop.
*Accept:* with a Corbie set up by `corbie init --ceiling all`, `corbie start controller runner bulldog` serves the house; stopping the runner stops Bulldog.

**R-server-10** Another Corbie is named as `corbie` itself is told: the config by `--config` or `CORBIE_CONFIG`, the store by `--store` or `CORBIE_STORE`; what is on the command line is used in place of the environment, and that in place of what the config recorded; either may be written before the folder or after it.
*Accept:* with `CORBIE_STORE` naming one store and `--store` another, the server is on the second. `corbie bulldog --store <s> <folder>` and `corbie bulldog <folder> --store <s>` serve the same.

**R-server-11** The server says which Corbie it is on, by its controller's name and its store's address, before anything else it serves.
*Accept:* the first line after readying names the controller and the store, before the office's line.

**C-server-07** A store's address is printed without any user, password or setting it carries, an address that cannot be read is never echoed, and no error repeats one. Store addresses carry credentials, and the start line ends up in logs. *Test:* `server_store_address_shown_without_what_it_carries`

**C-server-08** A config whose client has no identity of its own is refused before anything is served, saying what to run; so are a client that trusts no controller and a store this Corbie cannot read, as Corbie refuses them. Bulldog starts runs, and a client that can only look cannot. *Test:* `server_viewer_refused`

**R-server-12** Bulldog needs nothing else of Corbie: no gateway, no token, and no address of its own that jobs call.
*Accept:* the house is served on an in-process Corbie, and a request runs to its answer, with nothing listening for jobs and no token set.

## The demo

**R-server-54** `mise run demo` serves the house the office page shows, with its Telegram bridge, on a Corbie in the same process, with the tests' stand-in model and a stand-in for the Telegram bridge; it needs no network, no key and no Corbie set up on the computer. The stand-in plays the house's workers through the house's day, as the [Server overview](../server.md) tells it. The demo opens with the first part of the day played, as Sam and Kit would on the screens, each step once the one before it has settled: Kit's socket, with a photo of it, waiting on Kit after Sam's yes to cutting its circuit; Sam's fan, waiting on his yes to an order the small-orders rule passed to him; Kit's question about the lawn, answered through a sub-request to the garden; and the bins routine, run now by Sam. What stops the opening is said in the log, and the house is served all the same.
*Accept:* on a computer with no network, no Corbie set up, and no `CORBIE_` variable or key set, `mise run demo` prints the links, and opens with the day's requests of the house as they stand. Kit's socket holds a PNG on her entry, has the cut decided yes by Sam and the order said yes to by the small-orders rule, and waits on Kit, whom the fixer asked to say when the new socket arrives; the cut's decision started the stand-in Telegram bridge's out run, with Sam's Telegram identity as where it goes. Sam's fan waits on Sam, whose decision carries the rule's pass and its note, that £48 is more than it orders alone. Kit's lawn is answered with the groundsman's words, and has one sub-request, to the garden, answered. The bins routine's request is answered. Kit then asks whether the kettle is on, and is answered with the line saying what the demo plays. Each worker, given its part turn by turn, plays it as the overview tells it, a refused cut, a refused order and a garden that does not answer among them; and a word that only holds another, such as the power, the shower or a fancy light, starts no part.

## Where it is served

**R-server-13** The server listens on `--listen`, or else `BULLDOG_LISTEN`, or else `127.0.0.1:7410`.
*Accept:* with `BULLDOG_LISTEN=127.0.0.1:7499` and `--listen 127.0.0.1:7498`, it listens on 7498; with neither, on 7410.

**R-server-14** The office's url is `--url`, or else `BULLDOG_URL`, or else `http://` and the listen address; every link the office gives is made with it.
*Accept:* with `--url https://house.example`, the printed links begin `https://house.example/?key=`, and a one-time link sent by a bridge begins `https://house.example/signin/`.

**C-server-09** No link is made with the listen address when a url is given. Behind a proxy, or under a name of its own, the listen address is not how a person reaches the office. *Test:* `server_links_use_url`

**C-server-10** Of the environment, Bulldog reads for itself only names that begin `BULLDOG_`, beside Corbie's own. A variable left in the shell for something else must never be taken as the office's. *Test:* `server_only_bulldog_variables_read`

**R-server-15** The listen address and the url are read once, at the start, and are in no file of the office.
*Accept:* the house's `office.yaml` with a `listen` key is a problem of the reading; a server started with `--listen` on one port and restarted with another listens on the second.

**R-server-16** The server speaks plain HTTP, and takes a request as having come over TLS when it did, or when a proxy in front says so with `X-Forwarded-Proto: https`.
*Accept:* a request with `X-Forwarded-Proto: https` is taken as having come over TLS; the same without the header is not.

## Starting and stopping

**R-server-17** A start goes in this order: the folder is readied; the files Bulldog ships are written; Corbie's client and the store are opened; Corbie is looked at for another server of the office (R-server-53); the office is read; the address is taken; and then the server's own run, watching Corbie, the bridges, the clocks and the reading of the office again are started, and the links printed.
*Accept:* with the house served, the log's lines come in that order, and the first run started, the server's own, is started after the listening line.

**C-server-11** The listen address is taken before any work starts: an address that cannot be taken starts no run, no bridge, no clock and no watch, sends nothing, and prints no link, and the server ends with a failure that names the address. A server nobody can reach must not do work nobody can see or stop. *Test:* `server_listens_before_any_work`

**R-server-18** An office that does not read well at its start is served all the same: every problem is in the log, with its file and what is wrong, no earlier reading is used, it starts nothing but the server's own run (R-server-53), and nobody signs in until it reads well.
*Accept:* the house is served, stopped, and its `office.yaml` is left with no `lead`; served again, the log names the problem in `office.yaml`, `/?key=` with any word is refused, and no run is started but the server's own. Writing `lead: steward` gives a reading after which the links are printed and Sam signs in. Served instead with `bridges/telegram/out.yaml` removed, the office reads well: the log names the problem with the Telegram bridge, the links are printed, Sam signs in, Kit's request in the chat runs to its answer, and no run of the Telegram bridge is started.

**C-server-12** An office that has read well and then does not is served as it was last read well, its screens working and the problem shown. A half-saved edit by hand must not take the office down. *Test:* `server_bad_edit_keeps_serving`

**R-server-19** The log says which Corbie; the office's name with how many teams, workers and people it has; every problem of the reading, with its file; and a link for each person. Each later reading adds every problem new in it, and a link for each person new in it. A link is printed only once every area has taken the reading it comes from, so a request made with it at once is taken.
*Accept:* the house prints "The house: 2 teams, 8 workers, 2 people" and a link for `sam` and one for `kit`. `jo` written into `everyone` while it is served prints one link, for `jo`, and none again for the others. Kit's `POST /api/requests`, made as soon as her link is printed, is taken.

**R-server-20** A signal stops the server: it stops listening, stops everything it started, waits for all of it to end, closes the store, and ends with status 0.
*Accept:* the house is served with a request running; the server is sent an interrupt; it ends with status 0, the store closed and nothing of it still going.

**C-server-13** Nothing the server started reads or writes the store, the office or Corbie after the store is closed. Parts left running after a stop would write into a closed store. *Test:* `server_shutdown_waits_for_everything`

**C-server-14** A server stopped by a signal ends as having done what it was asked, not as having failed. Corbie's own program takes a cancelled run of a command for a failure. *Test:* `server_stopped_by_signal_succeeds`

**C-server-15** Nothing the server needs is kept only in memory: keys, one-time links not yet used, and the identity are in `.bulldog/`. It can be stopped at any moment and started again, and a person signed in stays signed in. *Test:* `server_sign_in_outlives_restart`

## Bulldog's own folder

**R-server-21** `.bulldog/` at the top of the office holds the office's identity, the store and the files Bulldog ships; it is made when the folder is readied, is not the office, and is never in its history.
*Accept:* after `init` of an empty folder, `.bulldog/` holds `identity`, `bulldog.db`, `ask.yaml`, `call.yaml`, `live.yaml` and `clock.yaml` at its top; `tools/talk/`, `tools/notes/` and `tools/build/`, each with its tools file and its `README.md`; and `bridges/telegram/`, `bridges/slack/`, `bridges/gitlab/` and `bridges/jira/`, each with its machines, its script, its `settings.yaml` and its `README.md`; and the repository's commit holds none of them.

**R-server-22** The files Bulldog ships are written at every start, whole, as the program has them, in place of what is there, so they name exactly the tools the work area answers (C-work-23). The folders of the things it ships are written afresh, and the tools files earlier versions shipped flat at the top, `talk.yaml`, `notes.yaml` and `build.yaml`, are removed; nothing else in `.bulldog/` is touched.
*Accept:* `.bulldog/tools/talk/talk.yaml` is changed by hand, `.bulldog/talk.yaml` and `.bulldog/tools/talk/old.yaml` are written, and the server started again; `talk.yaml` in its folder is again exactly as the program has it, the other two are gone, and the identity and the store are as they were.

**C-server-51** A file Bulldog ships is never kept as it was found, whether changed by hand or left by another version of the program: every one is written again at each start, before the office is read, and a file in a shipped folder, or a flat tools file, that the program no longer ships is removed. A shipped file that named a tool the program does not answer would fail only when a model called it, and one left from an earlier version would be read by people as what is shipped. *Test:* `server_shipped_files_match_program`

**R-server-23** The office's identity is made once, at random, when the folder is first readied, and is the same at every start after.
*Accept:* the house is served, stopped, renamed in `office.yaml` and served again: the identity is the same, and so is the label on its runs. Two folders given the example have different identities.

**C-server-17** The identity is never made from the office's name or anything in its files. Otherwise two offices of one name on one Corbie would answer and cancel each other's runs, renaming an office would lose the runs it had, and a name of no letters would make an empty label. *Test:* `server_identity_not_from_name`

**C-server-18** The identity is kept beside the store and not in it, so a store made again keeps it. A new identity with a new store would leave the office's runs in Corbie under a label that nothing looks for. *Test:* `server_identity_outlives_store`

**R-server-24** The store is one file, `.bulldog/bulldog.db`, which the server opens once and each area keeps its own part of.
*Accept:* with the house served and a request asked, `.bulldog/` holds one database file, and the request, the key of each person and the identity's runs are all found after a restart.

**R-server-52** The office's identity records the folder it was made in, by its real path with links followed; a folder whose `.bulldog/` was made in another is refused by the server, as it is readied and before its store is opened, naming both folders and saying to remove `.bulldog/` for an office of its own or to run `corbie bulldog init` if the office was moved; `init` there takes the move, keeping the identity, the store and its runs, and says so; `check` reads it as it reads any office.
*Accept:* the house is served, stopped and copied with its `.bulldog/`; serving the copy ends with status 1 and a line naming both folders, the store's file is unchanged and no run is started. `corbie bulldog check` of the copy lists it as it lists the house. The house is moved to another folder: serving it is refused the same way; `corbie bulldog init` there says it took the move, and serving it then finds the request asked before the move and the same label on its runs.

**C-server-49** A folder is served only under an identity made in it, or taken there by `init`; a copy made with its `.bulldog/` is never served as the office it was copied from without a person saying so. Two folders under one identity on one Corbie answer and cancel each other's runs, and a working copy made beside the office in use is how that happens. *Test:* `server_copied_office_refused`

**R-server-53** While it serves, the server keeps one run of its own, of the `live.yaml` it ships, labelled with the office's identity: it renews the run every twenty seconds, the run ends itself when a minute passes without a renewal, and a server that stops ends it. A server that finds such a run of its office at its start, before it takes its address, waits for it up to a minute: renewed, the start is refused, naming the controller and saying to stop the server already serving the office, or, for a copy meant as an office of its own, to remove its `.bulldog/`; ended, the start goes on; neither, once its minute and Bulldog's thirty seconds of looking again have passed, the start is refused, saying no controller is running it.
*Accept:* the house is served, and served again from the same folder: the second ends with status 1 and that line, takes no address and starts no run, and the first serves on. The first is stopped, and the house is served again at once: it does not wait. The first is killed instead: a start waits, and goes on once the test's clock has moved a minute and the run has ended itself.

**C-server-50** Whether another server is serving the office is read from Corbie, by the office's identity, never from the store or the folder. One folder served twice shares its store and its folder, and two servers of one office each cancel the runs the other noted and answer the other's tickets. *Test:* `server_second_server_of_office_refused`

**C-server-19** A store made by an older Bulldog is refused, saying so and that removing it starts the office again, and the server does not start. Nothing is kept backwards compatible before version 1, and reading an older store would quietly do the wrong thing. *Test:* `server_older_store_refused`

## Signing in

**R-server-25** Every person has a key, made when they are first in a reading of the office and kept in the store.
*Accept:* the house is served; Sam and Kit each have a key. `jo` is added to `everyone`; at the next reading Jo has one.

**R-server-26** `GET /?key=` with a person's key gives the browser the key as a cookie, and answers with a page that moves on to `/`, whose address holds no key.
*Accept:* `GET /?key=<Sam's key>` answers 200 with a `bulldog_key` cookie holding Sam's key and a page that moves to `/`; `GET /` with that cookie is Sam's. `GET /?key=wrong` sets no cookie and shows the sign-in page.

**C-server-20** A key is taken from the cookie, and from an address only on `GET /`. A key in an address is kept in the browser's history, in logs and in what is sent to the next site. *Test:* `server_key_only_in_cookie`

**C-server-21** A sign-in answers with a page that moves on, never with a redirect. A strict cookie is not sent on a navigation another site began, so a link opened from Slack would land without the key. *Test:* `server_sign_in_moves_on_by_page`

**C-server-22** The key's cookie is HttpOnly, SameSite=Strict, kept for 400 days, and Secure exactly when the request came over TLS. A cookie given over a plain connection cannot be held to TLS, and one held to TLS whatever the connection is never sent back on a computer of one's own. *Test:* `server_cookie_secure_when_tls`

**C-server-23** A key is checked so that how long the check takes says nothing of how near a guess came. Otherwise a key could be found one character at a time. *Test:* none: a test cannot measure time finely enough to prove it; it is kept by review.

**C-server-24** A good reading of the office that no longer names a person destroys their key, and someone written later under the same name is given a new one; a reading that fails destroys nothing. A key that outlived its person would sign a stranger in as them. *Test:* `server_reused_name_gets_new_key`

**R-server-27** A person may replace their own key: every browser that held the old one is signed out, and the one that asked is given the new one.
*Accept:* Sam is signed in on two browsers and replaces his key from one; that one is still Sam's, the other is answered 401, and Sam's printed link of before is refused.

**R-server-28** The sign-in page takes a name; a link that works once, for ten minutes, is sent to that person's direct place on the first of their bridges, in the order their entry names them, that sends it: the first bridge's out machine is started once, and only if its run ends without sending is the next started; a bridge with no direct place is given no run, and the next is tried; and the page answers the same whatever name it was given.
*Accept:* the house, served as a test serves it, with a stand-in for its Telegram bridge as the demo has (R-server-54); Sam's name is given on the sign-in page: the bridge's out machine is started once, with a direct message to Sam's Telegram identity holding a link under `/signin/`. Sam is also written on a second stand-in bridge of the test's, after `telegram`, and, a minute later, the Telegram out machine refuses his next link: the second bridge's out machine is then started once, to Sam's direct place there, and Telegram's is not started again. A stand-in bridge with no direct place, written first, is given no run, since it sends nothing to anyone directly, and the next is started. `nobody` is given: no out machine is started, and the page's answer is the same as for Sam.

**C-server-52** A one-time link is no entry of any request: it is said in no thread, counted in nothing that waits, and shown on no screen. Whoever can read a request would otherwise be handed a way to sign in as someone else. *Test:* `server_sign_in_link_in_no_request`

**C-server-25** The sign-in page's answer and its time say nothing of whether a name is a person's. The page is open to anyone, and must not tell them who is in the office. *Test:* `server_sign_in_page_says_nothing_of_who_is_there`

**R-server-29** A one-time link, opened, shows a page; pressing on it signs the person in, as `/?key=` does, and uses the link up; a link used, or older than ten minutes, signs nobody in.
*Accept:* Sam's link is opened, then pressed: Sam is signed in. Pressed again, it is refused. Another link, pressed once the test's clock has moved eleven minutes on from its sending, is refused.

**C-server-26** A one-time link is used up only by pressing on its page, never by opening it. A service that carries a link opens it to show a preview, and would use it up first. *Test:* `server_one_time_link_survives_preview`

**C-server-27** At most one sign-in link is sent to a person in a minute; asking again sooner sends nothing, and the page answers as it always does. The page is open to anyone, who could otherwise fill a person's messages. *Test:* `server_sign_in_links_limited`

**C-server-28** Signing in uses no service outside the office, and a link goes only by the office's own bridges. Bulldog speaks to no outside service. *Test:* none: every test runs with no network, so a sign-in that needed a service outside would fail them all.

**C-server-29** Only a person written in the office has a key or signs in; a guest never does. A guest may ask and be answered where they asked, and is shown nothing on the screens. *Test:* `server_guest_never_signs_in`

**R-server-55** Where a person is reached, the bridges their entry names and the identity each gives for them, is changed only by a change to the office's files, decided like any other, their own entry included; no request of the API sets it, and nothing said on a bridge does. A one-time link goes only where the files say at the time it is asked for.
*Accept:* in the work office, Raj's `POST /api/office/check` of a change to his own Slack identity is refused with a sentence that says to ask for it. Sam's `POST /api/office/check` in the house of a change to his own Telegram identity gives it among what the change lets more by. Once a change replacing Kit's Telegram identity with another is written, a link asked for Kit is sent to the new identity and never to the old.

**C-server-53** Nothing but a decided change to the files sets where a person is reached. Whoever could set a person's identity on a bridge could have that person's one-time link sent to themselves, and take over their account. *Test:* `server_reach_changed_only_by_a_change`

**R-server-30** A request without a key is answered 401 with `{ "error": … }`, a sentence saying how to sign in; `GET /` without one shows the sign-in page.
*Accept:* `GET /api/me` with no cookie answers 401, JSON, with a sentence; `GET /` with no cookie answers the sign-in page.

## What a person sees

**R-server-31** Everyone signed in sees the office's name; each team with what it is called, its `about`, its people and its lead, and the team it is within; and each person with their name, their title and the bridges that reach them, by the bridge's name.
*Accept:* in the work office, Dana's `/api/me` holds the four teams, payments with its `about`, `lee` and `priya` and its lead `ledger`, support with its `about`, and Raj with his title and `slack` and `gitlab`; it holds no worker of payments but its lead, no Slack identity and no team's notes.

**R-server-32** A team's people, and the people who decide a change, see the rest of how it is written: its team file as read; each of its workers with its type, its model as it is sent, a model named by one word shown filled in from the nearest team that gives it, and its tools, each with its `does` as written and what its job is, a worker of the same team with that worker's name, another machine or a script with its file's path in the office, a machine written in place with neither, or one of the tools Bulldog ships; a `list:` entry as the tools its server lists, named after it; which of them wait for a yes, and who decides each, in order, each as the team file names them, with whether it is a person, a rule or a worker, and a worker with the team it is of; its routines; its notes; and its problems. Which of its workers are at work, and on what, is seen by its people alone.
*Accept:* Lee's `GET /api/teams/payments` holds the ledger with its model and its refund tool waiting for a yes from `lee`, then `priya`, the disclosure check, the notes, and the ledger at work on the sub-request; with the models of R-server-03's check, the ledger's model is the one payments gives `usual`, not the office's. Dana's holds what `/api/me` gives of payments, and nothing more. Al, who decides a change, is given the files, routines, notes and problems, and not what the ledger is doing. In the house, Sam's `GET /api/teams/office` holds the fixer's `sparks` as the worker sparks, in `sparks.yaml`; its `cut_power` as the script `tools/cut-power.js`, waiting for a yes from `sam`, and its `order_part` from `./small-orders.js`, then `sam`; and the steward's `fixer` as the worker fixer and its `ask_team` and `write_notes` as Bulldog's, each with its `does` as the file Bulldog ships writes it. With `order_part` written `[./small-orders.js, reviewer, sam]`, the reviewer a worker of the garden, Sam's `order_part` holds the rule, the reviewer as a worker of the garden, and Sam as a person; and a £300 order the reviewer passed is given to Sam with the reviewer as a worker of the garden.

**C-server-54** What a tool's job is, is told from where it resolves from the worker's folder, never from the tool's name: a tool is one Bulldog ships only when its job is the call Bulldog ships, `.bulldog/call.yaml`; a tool whose job is any other file in `.bulldog/`, such as the clock, is a machine. A tool of a worker's own named like one of Bulldog's would otherwise be shown as Bulldog's, and taken by whoever reads the team for a run that only waits on Bulldog. *Test:* `server_tool_told_by_where_it_resolves`

**C-server-30** Whether a worker is at work, and on what, is shown only to its team's people; to anyone else a worker at work on something they may not see is not shown at work. Showing it busy would say that something they may not see is going on. *Test:* `server_busy_worker_shown_to_its_team`

**R-server-33** A person sees the requests they may see, by team and by state; those they asked; what waits on them, and how much; and, of each request, what was done and what was read of each team they are in, as the requests area says.
*Accept:* in the work office after the refund, Dana's `GET /api/requests?asked=me` holds her request; `GET /api/requests/{n}` of it holds both sub-requests, "Refund one charge: yes from Lee, 14:02" and both answers, and no ledger line. Lee's `GET /api/requests/{n}` of payments' sub-request holds the ledger's lines.

**C-server-31** Who sees a request, which layer of it, and what waits on whom are asked of the requests area, and the server works none of them out itself. Worked out in the screens and the store apart, they would drift, and a person be shown a count the list does not hold. *Test:* none: it is a rule of the code's shape, kept by review; the requests area's tests hold what it answers.

**R-server-34** A run is watched only by the people of the team its labels name, who are those that see what was read of it (R-requests-36). A worker asked to decide for another team runs as the team it decides for (C-decisions-23), and is watched by that team's people, not its own team's. A run of a team that has gone, or been renamed, is watched by nobody.
*Accept:* Lee watches the ledger's run; Al's watch of the same run is answered as not there. Al watches platform's run, since platform has no people and its decisions reach him; Dana's watch of it is answered as not there. With `decide: { everything: [ops, lee, priya] }` in `payments/team.yaml`, the ops run asked to decide the refund is watched by Lee, and Al's watch of it is answered as not there. Platform's folder is renamed: Al's watch of its earlier run is answered as not there.

**R-server-35** The office's files, which of them are edited by hand, and the history (C-changes-45) are seen only by the people who decide a change, who also see every team's notes and every problem; a team's own people see its notes and its problems (R-server-32, R-server-37), and nobody else sees any of them.
*Accept:* in the house, Sam's `GET /api/office/files` holds every file of the office, the notes included, with those edited by hand marked, and his `GET /api/history` every change; Kit's are answered 404, since the house writes no `change` list and only Sam, on its `everything`, decides a change to it. With `bins.yaml` broken, Kit's `GET /api/teams/office` holds the house's notes and that problem, and her `GET /api/teams/garden` holds neither the garden's notes nor its problems.

**R-server-36** The top team's people see each bridge that is failing, with what Corbie said.
*Accept:* the work office's stand-in Slack in machine fails; Al's `/api/me` holds the failure, and Dana's does not.

**R-server-37** A person sees the problems of the teams they are in, and the people who decide a change see every problem.
*Accept:* with `ledger.yaml` broken, Lee's `/api/me` holds the problem; Dana's does not; Al's does. With the GitLab bridge's folder removed, the problem naming the GitLab bridge is the office's: Al's `/api/me` holds it and Dana's does not, and Dana's request from #help runs to its answer.

## What a person may do

**R-server-38** A person asks in the chat, of the office or of any team they name, with files or none.
*Accept:* Kit's `POST /api/requests` with the garden named makes a request to the garden, Kit its asker and the chat its origin; one with no team goes to the house.

**R-server-39** A person says more where the requests area says they may (R-requests-19): in a request they asked, one whose team they are in, one whose origin they can read, or a sub-request asked by a team they are in; the same on the screens as through the bridge, whether the request is open or answered. They answer a question put to them, in the request it is in.
*Accept:* Kit says more in her request; Sam, in the house, says more in it too. Raj, who reads #help, says more on the screens in Dana's request, and it is an entry to the office. Al, of the office whose bot asked payments, says more in payments' sub-request, and it is an entry to payments. Dana, who sees payments' sub-request only from her request, saying more in it is refused with a sentence. Lee, who sees only payments' sub-request, saying more in Dana's request is refused as not there. The fixer's question to Kit is answered by Kit's `POST /api/requests/{n}/entries` naming it.

**R-server-40** A person decides a decision the walk reaches them for, while it waits, with yes or refuse and a note, and is shown the note of each decider that passed it on.
*Accept:* Lee's `POST /api/requests/{n}/decisions/{id}` with `{ "says": "yes" }` on the refund is taken as Lee's yes; Dana's on the same is refused with a sentence. In the house, an order of £300 that `small-orders.js` passed with a note reaches Sam with that note.

**R-server-41** A person stops a request they asked, one whose team they are in (for a team with no people, one whose decisions reach them, R-requests-36), or one beneath a request they may stop.
*Accept:* Al stops Dana's request; Lee stops payments' sub-request; Dana stops her own. Raj's stop of payments' sub-request is refused with a sentence. Raj asks platform in the chat: Al stops that request, and Dana's stop of it is refused.

**R-server-42** A team's people run one of its routines now, named by what is written in it.
*Accept:* Sam's `POST /api/teams/office/routines/now` with `{ "at": "mon 09:00", "to": "steward", "say": "Walk round: what is still open?" }` opens the routine's request, which keeps that Sam ran it (C-server-32). Kit, not in the garden, running a garden routine is answered 404, as is a routine written otherwise than the body says.

**R-server-43** The people who decide a change draft one on the screens, setting a bridge Bulldog ships by its entry if they choose, have it checked, and put it; it still goes first to every rule and worker the walk asks before it reaches a person, as the changes area says (R-changes-29). Anyone else asks for one in a request.
*Accept:* Sam's `POST /api/office/check` with a new worker's file gives the files, the problems, what the change says and what it lets more by, and writes nothing; in the house as the server writes it (R-server-06), with the field `bridges.telegram` of `office.yaml` set to `{ from: ./.bulldog/bridges/telegram, places: { direct: {} } }`, the settings Bulldog ships it with, it gives `office.yaml` alone, with `telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }` under `bridges`, and can be put. Kit's is refused with a sentence that says to ask for it.

**C-server-32** Every action on the screens is done as the person signed in, and is kept as theirs. An action that carried nobody could be done by anyone with a key, and would be answered as no one's. *Test:* `server_every_action_has_its_person`

## The API

**R-server-44** The API is JSON in and out; a request or an entry with files is a form of several parts, the text and each file in one body.
*Accept:* Kit's `POST /api/requests` as a form with a text and a photo makes one request whose first entry holds both.

**R-server-45** The API's requests are these:

| Request | Gives or does |
|---|---|
| `GET /api/me` | `{ me, office, teams, people, waiting, problems, failing }`: who is looking, with the teams they are in and whether they may decide a change; the office's name and `zone`, in which the screens show every time; each team, with its `about`, and each person as everyone sees them; how many things wait on them; the problems they may see; and, to the top team's people, each bridge that is failing |
| `GET /api/requests?team=&state=&asked=me&waiting=me` | the requests the one looking may see, newest first, each with its number, team, asker, origin, and state, `working`, `waiting` or `answered`; for one from an issue, its source's status as last reported, with when, in place of a state, and whether its work is going or waits on someone; and what it waits on |
| `GET /api/requests/{n}?before=&size=` | one request: what was done, and what was read of each team the one looking is in; its sub-requests as they may see them; its thread in part, with `before` naming the entry to reach back from and `size` how many entries; the decisions the one looking was asked, with the note of each decider that passed them on, and the team of each worker among them; and its runs, to the team's people |
| `GET /api/teams/{team}` | one team as the one looking may see it |
| `GET /api/office/files`, `GET /api/office/file?path=` | the office's files by path, and one file's text |
| `GET /api/history`, `GET /api/history/{id}` | every change, newest first, with who, why and who decided; and one change with each file it touched, before and after |
| `GET /api/events` | the stream of changes |
| `GET /api/runs/{run}/watch` | the stream of one run, its name escaped as one part of the address; a screen that connects again gives `Last-Event-ID` |
| `GET /files/{entry}-{i}` | the file of an entry at its place among the entry's files |
| `POST /api/requests` `{ team, text }` | asks in the chat; gives the request |
| `POST /api/requests/{n}/entries` `{ text, re }` | says more in it; with `re`, an open question to the one looking, answers it |
| `POST /api/requests/{n}/decisions/{id}` `{ says, note }` | decides: `says` is `yes` or `refuse` |
| `POST /api/requests/{n}/stop` | stops it, and what is beneath it |
| `POST /api/teams/{team}/routines/now` `{ at, to, say }` | runs the routine written so |
| `POST /api/office/check` `{ files, set, move, remove }` | a draft: files written whole, or `null` to remove one; a field set in a file; a worker moved to a team; a worker taken away. Gives `{ files, problems, says, lets_more_by, mark }`, and writes nothing; the `mark` names what was checked |
| `POST /api/office/put` `{ why, mark, files, set, move, remove }` | puts the draft to be decided, with the `mark` of the check the person was shown: a draft or an office not as it was checked is not put, and they are told to look again. Gives what became of it |
| `POST /api/me/key` | replaces the one looking's key |

and these, open without a key:

| Request | Gives or does |
|---|---|
| `POST /signin` `{ name }` | asks for a one-time link; answered the same whatever the name |
| `GET /signin/{link}` | the one-time link's page, which uses nothing up |
| `POST /signin/{link}` | signs the person in, as `/?key=` does, and uses the link up |
| `POST /bridges/{name}` | a delivery to a bridge that hears, handed unread to a new run of its in machine |

*Accept:* each request in the tables, made by Sam on the house as the table says, answers with what its row says; each of the second table answers the same with no cookie.

**C-server-33** Each request finds the one thing it is about and checks the one looking's sight of that thing, from the files then. Sight guessed from the words in an address would take one kind of number for another, refusing a person what is theirs while letting by what is not. *Test:* `server_sight_checked_on_the_thing_named`

**C-server-34** A thing the one looking may not see is answered exactly as one that does not exist: 404, with the same sentence. A refusal that differs tells them it is there. *Test:* `server_hidden_is_not_there`

**C-server-35** Something within a request, an entry or a decision, is acted on only through the request that holds it; named under another request it is not there. Otherwise sight of one request reached into another by number. *Test:* `server_entry_only_through_its_request`

**R-server-46** Every answer that is not a success is JSON, `{ "error": … }` with a sentence a person can read: 401 without a key; 400 for a body that cannot be read; 404 for what is not there; 409 for what the rules refuse; 413 for a body too large, naming the limit; 500 for a failure, which is also in the log. A one-time link is pressed from a page of its own, in the browser, so a link that signs nobody in is answered 404 with a page saying so, in place of JSON.
*Accept:* a malformed body answers 400, a refused stop 409, a body of 2 MiB to `/api/requests/{n}/entries` 413, and a store made to fail 500 with its line in the log; every one of them JSON with `error`. A one-time link pressed once it is used answers 404 with a page saying it signs nobody in.

**C-server-36** A failure is answered as a failure, never as a refusal or as a thing not there. A failure answered as a refusal would hide what is wrong from the person who could mend it. *Test:* `server_failure_not_refusal`

**C-server-37** A body is read whole and exactly: one that is not well formed, holds a field the request does not take, or gives a field of the wrong kind is refused, and a field not sent is never taken as empty. A body read as empty would run as if nothing were asked, and a field left out would blank what it names. *Test:* `server_body_read_strictly`

**C-server-38** A body over its limit, 1 MiB of JSON or 32 MiB of a form with files, is refused whole, naming the limit, and nothing of it is kept. A body cut short would be kept as if it were whole. *Test:* `server_too_large_refused_never_cut`

**R-server-47** A request's thread is given in part: its latest 50 entries, or those before the entry `before` names, at most 200 at a time, with the entry to name to reach further back when there is more.
*Accept:* a request of 300 entries gives its latest 50 and the entry to reach back from; with `before` that entry, the 50 before it; with a size asked of 500, 200.

**C-server-39** A thread given in part always holds its first entry and every entry earlier than the part that still waits on the one looking. A question to a person must not be hidden behind the part not shown. *Test:* `server_waiting_entry_never_hidden_by_paging`

**R-server-56** A list of requests is given in part, newest first: the newest 100, or as many as `limit` asks for, at most 500, of those numbered below the request `before` names, or of all; with the number to give as `before` for the next part, when more are left. A `limit` or a `before` that is not a number above nothing, or is given twice, is answered with a sentence.
*Accept:* Raj has asked 620 requests: `GET /api/requests?asked=me` gives the newest 100 and, as `before`, the hundredth's number; with that `before` and a `limit` of 1000, the next 500 and a `before`; with that, the last 20 and no `before`. A `limit` of `lots` or 0, a `before` of -3, and a `limit` given twice are each answered 400 with a sentence.

**C-server-55** A part of a list of requests is cut by request number, never by a place in the list. A request made between two parts would otherwise move every later part on by one, and a screen would show one request twice. *Test:* `server_request_pages_cut_by_number`

## The streams

**R-server-48** `GET /api/events` is a stream of server-sent events: `hello` when it connects, then `change` naming what changed, `request:<n>`, `waiting` or `office`, and a comment every 25 seconds when nothing else is sent.
*Accept:* a screen connects and is sent `hello`; Kit asks something and Kit's stream is sent `change` with `request:` and its number; the office is read again and every stream is sent `office`; a comment is sent once the test's clock has moved 25 seconds with nothing else to send.

**C-server-40** A stream names only what the one looking may see, with sight read when each is sent. Every change named to every screen would tell everyone the numbers of what they may not see. *Test:* `server_events_filtered_by_sight`

**C-server-41** A screen that connects is sent `hello` first, and fetches what it shows. Whatever changed while it was away would otherwise be missed. *Test:* `server_events_begin_with_hello`

**C-server-42** A change never waits on a screen. A screen whose stream has fallen behind misses what it could not take, and the next thing it is sent is `hello`. A slow browser must not stall whoever is writing, and a screen that missed a change must not go on showing it. *Test:* `server_slow_screen_never_blocks`

**C-server-43** Both streams are sent with `Cache-Control: no-store` and `X-Accel-Buffering: no`. A proxy in front would buffer a stream, and changes would come late or never. *Test:* `server_streams_not_buffered`

**R-server-49** `GET /api/runs/{run}/watch` is a stream of what one run of the office does, from its beginning: the model's words as they come, each tool call and its result, each log line, and each refusal with the line that would fix it; then `ended`.
*Accept:* Lee watches the ledger's run while it reads the charges: he is sent its words, its tool call with its input and its result, and `ended` when it ends.

**C-server-44** Each event of a run's stream carries its place in the run, and a screen that connects again with the last place it had is sent only what comes after it. A stream that began again from the start would show everything twice. *Test:* `server_run_watch_resumes`

**C-server-45** A run that does not carry the office's label is not there to watch. A run of another office on the same Corbie must not be shown. *Test:* `server_watch_only_this_offices_runs`

## Files and pages

**R-server-50** `GET /files/{id}` serves a file on an entry to whoever may see that entry.
*Accept:* the photo on Kit's request to the house is served to Kit and to Sam. In the work office, a file Lee puts on payments' sub-request is served to Lee and Priya, and Al and Dana are answered 404.

**C-server-46** A file is served with its type told from what it holds, never from its name, with `X-Content-Type-Options: nosniff` and a sandbox that lets it run nothing; only a picture other than SVG, or a PDF, is shown, and anything else is given as a download. A file put in must never run as a page of the office, where the key's cookie lives. *Test:* `server_files_served_harmlessly`

**R-server-51** The screens are served from the program: HTML, CSS, plain JavaScript and the fonts they are set in, which are served typed as fonts.
*Accept:* `GET /` with Sam's cookie answers the screens with no other site named in what it loads.

**C-server-47** The screens load nothing from any other site, and every page is served with a policy that allows nothing else. A page that loads from elsewhere hands that site the office's screens. *Test:* `server_screens_load_nothing_from_elsewhere`

**C-server-48** Only `GET /`, the sign-in pages, the screens' own files and the address of a bridge that hears are answered without a key. Everything else that is open is something nobody decided to open. *Test:* `server_only_sign_in_and_bridges_open`

## Interfaces

- **Office** is read from the folder the server readies, and gives each team's `about`, each worker's model with a one-word name filled in from the nearest team that gives it (R-server-03, R-server-32), and every problem of the reading, a problem of one worker or one bridge stopping only that one (R-server-18), which the server prints and shows to those who may see it (R-server-19, R-server-37).
- **Requests** keeps its part of the store the server opens, and says who sees what of each request, what was read of each team, who may say more (R-server-39), what waits on whom (C-server-31), and who are the people of a team with no people (R-server-34, R-server-41).
- **Work** is started once the server listens (C-server-11), labels every run with the identity the server holds (R-server-23), leaves the server's own run to the server (R-server-53), answers exactly the tools in the files the server writes (C-work-23, C-server-51), and stops a request at the word of a person the server says may stop it (R-server-41).
- **Decisions** is given each person's answer from the screens, as theirs (R-server-40, C-server-32), and says that a worker asked to decide for a team runs as that team's (R-server-34).
- **Bridges** carry a one-time sign-in link to a person's direct place, one out run at a time, and tell the server how each ended (R-server-28, C-server-52), and are called at an address open without a key (C-server-48).
- **Changes** is given a person's draft to check and put (R-server-43), names the people who decide a change (R-changes-38), and says who sees its history (C-changes-45); the server makes a repository for a folder that has none (R-server-07), and the office's first commit is the one commit Bulldog makes that is neither a decided change nor a team's note; a change to where a person is reached is decided as any change is (R-server-55).
- **Corbie's client** opens the store and starts every run; Corbie's own program reads the command line (R-server-02).
