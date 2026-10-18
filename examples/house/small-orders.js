// A rule: the house orders small parts on its own. It is given the
// decision, and says yes to anything under £20; anything more it passes
// to Sam, with why.
const pounds = Number(input.action && input.action.args ? input.action.args.pounds : NaN)
if (pounds < 20) return { says: "yes", note: "Under £20." }
return { says: "pass", note: "£" + pounds + " is more than I order alone." }
