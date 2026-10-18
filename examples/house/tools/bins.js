// Which bin goes out tonight: green on Tuesdays, blue on Thursdays, and
// nothing on other nights. The day is the day the routine came.
const entries = (input.request && input.request.entries) || []
const day = entries.length ? new Date(entries[0].time).getDay() : -1
if (day === 2) return "The green bin goes out tonight."
if (day === 4) return "The blue bin goes out tonight."
return ""
