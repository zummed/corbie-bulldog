// The screens start here, once every view has been added: anything else
// is the desk, and the stream of changes keeps what is shown current.
"use strict";

B.route(/^/, B.desk, "desk");

addEventListener("hashchange", async () => {
  B.said("");
  B.streaming = null;
  await B.show();
  scrollTo(0, 0);
});
document.getElementById("stale-show").addEventListener("click", () => B.show());

// The stream says hello when it connects, and again when it fell behind,
// and what is shown is fetched again then, as at any change.
const changes = new EventSource("/api/events");
changes.addEventListener("hello", () => B.changed(""));
changes.addEventListener("change", (e) => B.changed(e.data));
B.show();
