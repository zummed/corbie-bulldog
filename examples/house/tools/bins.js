// Which bin goes out tonight: green on Tuesdays, blue on Thursdays, and
// nothing on other nights. The day is the day the routine came: its
// asking opens the thread, among the entries or, on the routine's own
// run, new.
const request = input.request || {}
const thread = (request.entries || []).concat(request.new || [])
const day = thread.length ? new Date(thread[0].time).getDay() : -1
if (day === 2) return "The green bin goes out tonight."
if (day === 4) return "The blue bin goes out tonight."
return ""
