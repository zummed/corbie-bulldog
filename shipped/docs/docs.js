// Lists, reads or writes a team's docs, kept under input.place. A doc's
// name is one part of a kept path, such as refunds.md.
//   list    gives { names }, every doc's name, in order
//   read    gives { name, text }; text is null when the doc is not there
//   write   keeps text as the doc, whole, and gives { name }
const place = input.place
if (input.do === "list") return { names: kept.list(place).map(p => p.slice(place.length + 1)) }
const name = input.name
if (typeof name !== "string" || name === "" || name.includes("/")) {
  throw new Error(JSON.stringify(name) + " is not a doc's name: a name is one part of a kept path, small letters a to z, digits, ., - and _, not starting with a dot, such as refunds.md")
}
if (input.do === "read") return { name: name, text: kept.read(place + "/" + name) }
if (input.do === "write") {
  kept.write(place + "/" + name, { text: input.text })
  return { name: name }
}
throw new Error("do is list, read or write, not " + JSON.stringify(input.do))
