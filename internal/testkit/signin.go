package testkit

// SignIn gives the files of a stand-in sign-in method, by their paths in
// its folder, for an office of a test's own under signin/<name>/. Its
// machine reaches no provider: it answers the code it is given as the
// account, or refuses with what follows "refuse:" in it, or fails when
// the code is "fail", and its answer carries the nonce it was given, so a
// test sees what the run was given.
func SignIn() map[string]string {
	return map[string]string{
		"signin.yaml": `# A stand-in sign-in method, which reaches no provider.
flow: signin-standin
input: { code: string, verifier: string, redirect: string, nonce: string, settings: any }
stages:
  answer: { job: { script: ./answer.js, input: { code: "${input.code}", nonce: "${input.nonce}" } } }
output: ${stages.answer.output}
`,
		"answer.js": `if (input.code === "fail") throw new Error("the provider is not reached")
if (input.code.startsWith("refuse:")) return { refused: input.code.slice(7), nonce: input.nonce }
return { identity: input.code, nonce: input.nonce }
`,
	}
}
