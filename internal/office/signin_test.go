package office_test

import (
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

// signingIn gives the house, with Sam signing in by Google and by a
// password of his own, his account written in capitals and the client's
// ID a short one.
func signingIn(t *testing.T) map[string]string {
	t.Helper()
	files := replace(t, house(t), "office.yaml", "google: sam@example.com", "google: Sam@Example.com")
	return replace(t, files, "office.yaml", "    client_id: 1234567890-house.apps.googleusercontent.com   # from the Google Cloud console; Corbie keeps GOOGLE_CLIENT_SECRET\n", "    client_id: 1234-house.apps.googleusercontent.com\n")
}

// stopped reads an office that reads, with a problem stopping one sign-in
// method alone, and gives its problems.
func stopped(t *testing.T, files map[string]string, method string) office.Problems {
	t.Helper()
	o, problems := read(t, files)
	if o == nil {
		t.Fatalf("the office is refused: %v", problems)
	}
	of := problems.OfSignIn(method)
	if len(of) == 0 || o.SignIns[method].Runs() {
		t.Fatalf("%s signs people in, with the problems %v", method, problems)
	}
	for _, p := range of {
		if p.Team != "office" || p.Refuses {
			t.Errorf("the problem %v is not the office's, stopping %s alone", p, method)
		}
	}
	if !o.Bridges["telegram"].Runs() || slices.ContainsFunc(o.Everyone, func(name string) bool { return o.People[name] == nil }) {
		t.Error("a sign-in method's problem stopped more than it")
	}
	return of
}

func TestSignInMethodsInTheFiles(t *testing.T) {
	t.Parallel()
	o := reads(t, signingIn(t))
	m := o.SignIns["google"]
	if !slices.Equal(o.SignInOrder, []string{"google"}) || !m.Runs() || m.Dir() != ".bulldog/signin/google" || m.Line == 0 {
		t.Fatalf("the sign-in methods are %v, google running %s", o.SignInOrder, m.Dir())
	}
	if !reflect.DeepEqual(m.Settings, map[string]any{"from": "./.bulldog/signin/google", "client_id": "1234-house.apps.googleusercontent.com"}) {
		t.Errorf("google's settings are %#v", m.Settings)
	}
	// What its entry leaves out is the default its machine's input gives.
	if m.Setting("client_id") != "1234-house.apps.googleusercontent.com" || m.Setting("authorize") != "https://accounts.google.com/o/oauth2/v2/auth" || m.Setting("label") != "Google" || m.Setting("nothing") != "" {
		t.Errorf("google's settings read as %q, %q, %q", m.Setting("client_id"), m.Setting("authorize"), m.Setting("label"))
	}
	if !slices.Contains(o.Checked, ".bulldog/signin/google/signin.yaml") {
		t.Errorf("google's machine was not checked: %v", o.Checked)
	}
	// An office's own folder of the name is run when its entry has no from.
	own := signingIn(t)
	own["office.yaml"] = strings.Replace(own["office.yaml"], "    from: ./.bulldog/signin/google\n", "", 1)
	for name, text := range shippedFolder(t, "signin/google") {
		own["signin/google/"+name] = text
	}
	if o := reads(t, own); o.SignIns["google"].Dir() != "signin/google" || !o.SignIns["google"].Runs() || !slices.Contains(o.Checked, "signin/google/signin.yaml") {
		t.Errorf("the office's own google runs %s", o.SignIns["google"].Dir())
	}
	// The house without it, and Sam's account, reads with no sign-in
	// method; the work office signs in by entra.
	none := replace(t, signingIn(t), "office.yaml", "signin:\n  google:\n    from: ./.bulldog/signin/google\n    client_id: 1234-house.apps.googleusercontent.com\n", "")
	if o := reads(t, replace(t, none, "office.yaml", ", google: Sam@Example.com", "")); len(o.SignIns) != 0 {
		t.Errorf("the house has the sign-in methods %v", o.SignInOrder)
	}
	if o := reads(t, work(t)); !slices.Equal(o.SignInOrder, []string{"entra"}) || o.SignIns["entra"].Dir() != ".bulldog/signin/entra" || len(o.People["al"].Accounts) != 1 {
		t.Errorf("the work office signs in by %v", o.SignInOrder)
	}
}

// shippedFolder gives the files of a thing Bulldog ships, by their names in
// its folder.
func shippedFolder(t *testing.T, thing string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, name := range shipped.Files(thing) {
		contents, _ := shipped.File(path.Join(thing, name))
		files[name] = string(contents)
	}
	return files
}

func TestASignInMethodsProblemStopsOnlyIt(t *testing.T) {
	t.Parallel()
	files := signingIn(t)
	says(t, stopped(t, replace(t, files, "office.yaml", "./.bulldog/signin/google", "./.bulldog/signin/apple"), "google"), "office.yaml", "google's from", "./.bulldog/signin/entra")
	says(t, stopped(t, replace(t, files, "office.yaml", "    client_id: 1234-house.apps.googleusercontent.com\n", ""), "google"), "office.yaml", "google's settings", "client_id")
	says(t, stopped(t, replace(t, files, "office.yaml", "    client_id:", "    colour: blue\n    client_id:"), "google"), "office.yaml", "colour")
	says(t, stopped(t, replace(t, files, "office.yaml", "signin:\n  google:\n    from: ./.bulldog/signin/google\n    client_id: 1234-house.apps.googleusercontent.com\n", "signin:\n  google: yes\n"), "google"), "office.yaml", "mapping")
	says(t, stopped(t, with(files, map[string]string{"signin/google/signin.yaml": "flow: x\nstages: {}\n"}), "google"), "signin/google is not run")
	// The office's own: a folder with no entry, an entry with no folder,
	// a folder with no machine, and a machine that takes nothing it is
	// given.
	says(t, stopped(t, with(files, map[string]string{"signin/okta/signin.yaml": "flow: x\n"}), "okta"), "signin/okta", "does not name")
	nofrom := replace(t, files, "office.yaml", "    from: ./.bulldog/signin/google\n", "")
	says(t, stopped(t, nofrom, "google"), "no folder signin/google")
	says(t, stopped(t, with(nofrom, map[string]string{"signin/google/README.md": "Mine.\n"}), "google"), "no signin.yaml in signin/google")
	takesNothing := "flow: signin-google\ninput: { settings: any }\nstages:\n  one: { job: { script: ./answer.js } }\n"
	says(t, stopped(t, with(nofrom, map[string]string{"signin/google/signin.yaml": takesNothing, "signin/google/answer.js": "return { identity: 'x' }\n"}), "google"), "signin/google/signin.yaml", "code, verifier, redirect, nonce, settings")
	says(t, stopped(t, with(nofrom, map[string]string{"signin/google/signin.yaml": "flow: signin-google\nstages: [\n"}), "google"), "signin/google/signin.yaml")
}

func TestOfficeSignInMethodNamedAsNoBridgeIs(t *testing.T) {
	t.Parallel()
	files := signingIn(t)
	says(t, refused(t, replace(t, files, "office.yaml", "signin:\n  google:", "signin:\n  telegram:")), "office.yaml", "telegram is the name of a bridge too")
	says(t, refused(t, replace(t, files, "office.yaml", "signin:\n  google:", "signin:\n  password:")), "office.yaml", `"password" is a key of a person`)
	says(t, refused(t, replace(t, files, "office.yaml", "signin:\n  google:", "signin:\n  Google:")), "office.yaml", `"Google" is not a name`)
	says(t, refused(t, replace(t, files, "office.yaml", "signin:\n  google:", "signin:\n  signin:")), "office.yaml", `"signin" is a name Bulldog keeps`)
	says(t, refused(t, replace(t, files, "office.yaml", "  telegram:\n    from:", "  password:\n    from:")), "office.yaml", `"password" is a key of a person`)
	says(t, refused(t, replace(t, files, "office.yaml", "signin:\n  google:\n    from: ./.bulldog/signin/google\n    client_id: 1234-house.apps.googleusercontent.com\n", "signin: google\n")), "office.yaml", "signin names each sign-in method")
	// A folder under signin/ with a bridge's name, which signin does not
	// name, leaves a person's key of that name their identity on the
	// bridge, and says so.
	folder := with(files, map[string]string{"signin/telegram/signin.yaml": "flow: x\n"})
	says(t, stopped(t, folder, "telegram"), "signin/telegram", "telegram is the name of a bridge too")
	o, _ := read(t, folder)
	if sam := o.People["sam"]; !slices.Contains(sam.Reached, office.Identity{Bridge: "telegram", ID: "4471"}) || slices.ContainsFunc(sam.Accounts, func(a office.Account) bool { return a.Method == "telegram" }) {
		t.Errorf("with a folder signin/telegram, Sam is reached at %v and has the accounts %v", sam.Reached, sam.Accounts)
	}
}

func TestAPersonsAccountsAndPassword(t *testing.T) {
	t.Parallel()
	o := reads(t, signingIn(t))
	sam := o.People["sam"]
	if !slices.Equal(sam.Accounts, []office.Account{{Method: "google", ID: "Sam@Example.com"}}) || !sam.Password || !slices.Equal(sam.Reached, []office.Identity{{Bridge: "telegram", ID: "4471"}}) {
		t.Errorf("sam is %#v", sam)
	}
	if kit := o.People["kit"]; len(kit.Accounts) != 0 || kit.Password {
		t.Errorf("kit is %#v", kit)
	}
	// An account is compared whatever its case, and is nobody's by another
	// method.
	if p := o.Account("google", "sam@example.com"); p == nil || p.Name != "sam" {
		t.Errorf("sam@example.com by google is %v", p)
	}
	for _, a := range []office.Account{{"google", "kit@example.com"}, {"entra", "sam@example.com"}, {"telegram", "4471"}, {"google", ""}} {
		if p := o.Account(a.Method, a.ID); p != nil {
			t.Errorf("%s by %s is %s", a.ID, a.Method, p.Name)
		}
	}
	files := signingIn(t)
	says(t, refused(t, replace(t, files, "office.yaml", "password: true", "password: yes please")), "office.yaml", "sam's password is true")
	says(t, refused(t, replace(t, files, "office.yaml", "google: Sam@Example.com", `google: ""`)), "office.yaml", "sam's google", "empty")
	// A method the office does not have is no key of a person.
	says(t, refused(t, replace(t, files, "office.yaml", "google: Sam@Example.com", "entra: sam@corp.example")), "office.yaml", "sam gives entra, which is no bridge or sign-in method")
	reads(t, replace(t, files, "office.yaml", "password: true", "password: false"))
}

func TestOfficeAccountNamesOnePerson(t *testing.T) {
	t.Parallel()
	files := signingIn(t)
	says(t, refused(t, replace(t, files, "office.yaml", `telegram: "4472" }`, `telegram: "4472", google: SAM@example.com }`)), "office.yaml", "sam and kit are both SAM@example.com by google")
	// The same account by two methods is two accounts.
	files["office.yaml"] = strings.Replace(files["office.yaml"], "signin:\n", "signin:\n  google-work:\n    from: ./.bulldog/signin/google\n    client_id: 5678-house.apps.googleusercontent.com\n", 1)
	o := reads(t, replace(t, files, "office.yaml", `telegram: "4472" }`, `telegram: "4472", google-work: sam@example.com }`))
	if o.Account("google", "sam@example.com").Name != "sam" || o.Account("google-work", "sam@example.com").Name != "kit" {
		t.Error("an account names someone by another method")
	}
}
