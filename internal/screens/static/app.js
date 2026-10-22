// The screens by their addresses, then the start.
"use strict";

B.route(/^\/requests\/(.+)$/, B.requestView, "requests");
B.route(/^\/office$/, B.officeView, "office");
B.route(/^\/teams\/([^/]+)\/docs\/([^/]+)$/, B.docView, "office");
B.route(/^\/teams\/([^/]+)$/, B.teamView, "office");
B.route(/^/, B.home, "requests");

B.start();
