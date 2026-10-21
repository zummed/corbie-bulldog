package changes

import (
	"strings"
	"testing"
)

// signing gives the house, signing in by the Google Bulldog ships, with
// Sam's account by it and Sam able to set a password, its client's ID a
// short one.
func signing(t *testing.T) map[string]string {
	t.Helper()
	files := house(t)
	files["office.yaml"] = replaced(t, files, "office.yaml", "    client_id: 1234567890-house.apps.googleusercontent.com   # from the Google Cloud console; Corbie keeps GOOGLE_CLIENT_SECRET\n", "    client_id: house-client\n")
	return files
}

func TestChangesAWorkerCannotChangeHowAnyoneSignsIn(t *testing.T) {
	t.Parallel()
	files := signing(t)
	f := newOffice(t, files)
	for request, change := range map[int64][2]string{
		1: {"office.yaml", strings.Replace(files["office.yaml"], "google: sam@example.com", "google: thief@example.com", 1)},
		2: {"office.yaml", strings.Replace(files["office.yaml"], `telegram: "4472" }`, `telegram: "4472", password: true }`, 1)},
		3: {"office.yaml", strings.Replace(files["office.yaml"], "client_id: house-client", "client_id: another-client", 1)},
		4: {"office.yaml", strings.Replace(files["office.yaml"], "signin:\n", "signin:\n  entra:\n    from: ./.bulldog/signin/entra\n    tenant: 11111111-2222-3333-4444-555555555555\n    client_id: c\n", 1)},
		5: {"signin/okta/signin.yaml", "flow: okta\n"},
	} {
		f.write(request, change[0], change[1])
		check := f.check(request)
		if check.Fit() || !strings.Contains(strings.Join(check.Refused, "\n"), "a person's to write") {
			t.Errorf("a worker's change %d to how someone signs in is checked %+v", request, check.Refused)
		}
		if request == 5 {
			continue
		}
		// A person's is not refused, and is decided as any change is.
		drafted, _ := f.c.Drafted(ctx, request)
		mine, err := f.c.Check(ctx, drafted, Drafter{Name: "kit", Person: true})
		if err != nil || !mine.Fit() {
			t.Errorf("a person's change %d to how someone signs in is checked %+v, %v", request, mine, err)
		}
	}
}

func TestChangesSayWhatChangesInHowPeopleSignIn(t *testing.T) {
	t.Parallel()
	files := signing(t)
	f := newOffice(t, files)
	check := saysOf(t, f, map[string]string{"office.yaml": replaced(t, files, "office.yaml", `telegram: "4472" }`, `telegram: "4472", google: kit@example.com, password: true }`)})
	has(t, check.LetsMoreBy, "kit signs in as kit@example.com by google")
	has(t, check.LetsMoreBy, "kit may set a password to sign in with")
	check = saysOf(t, f, map[string]string{"office.yaml": replaced(t, files, "office.yaml", "google: sam@example.com", "google: sam@corp.example")})
	has(t, check.LetsMoreBy, "sam now signs in as sam@corp.example by google, where they were sam@example.com")
	// Taking them away narrows.
	check = saysOf(t, f, map[string]string{"office.yaml": replaced(t, files, "office.yaml", ", google: sam@example.com, password: true", "")})
	has(t, check.Says, "sam no longer signs in by google, where they were sam@example.com")
	has(t, check.Says, "sam may no longer sign in with a password")
	// A method new, or its settings changed, lets more by.
	check = saysOf(t, f, map[string]string{"office.yaml": strings.Replace(files["office.yaml"], "signin:\n", "signin:\n  entra:\n    from: ./.bulldog/signin/entra\n    tenant: 11111111-2222-3333-4444-555555555555\n    client_id: c\n", 1)})
	has(t, check.LetsMoreBy, "the sign-in method entra is new")
	has(t, check.LetsMoreBy, "entra's machine may connect to login.microsoftonline.com")
	check = saysOf(t, f, map[string]string{"office.yaml": replaced(t, files, "office.yaml", "client_id: house-client", "client_id: another-client")})
	has(t, check.LetsMoreBy, "google's settings are changed")
	check = saysOf(t, f, map[string]string{"office.yaml": replaced(t, files, "office.yaml", "signin:\n  google:\n    from: ./.bulldog/signin/google\n    client_id: house-client\n", "", "google: sam@example.com, ", "")})
	has(t, check.Says, "the sign-in method google is gone")
}
