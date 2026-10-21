package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// A person whose entry says password: true may set a password of their
// own once signed in, and sign in with it and their name. Only a slow,
// salted hash of it is kept, in the store. A wrong name and a wrong
// password are answered alike, and take as long: a name with no password
// is checked against a hash of nothing anyone knows.

// The limits on a password: its length, and how many wrong tries a name
// is given before each try waits, a minute after the last of them,
// doubling with each wrong try after, to an hour at most.
const (
	PasswordShortest = 10
	PasswordLongest  = 72
	Tries            = 5
	TriedWait        = time.Minute
	TriedLongest     = time.Hour
)

// The sentences a sign-in by password is answered with, the same
// whatever the name.
const (
	wrongPassword = "That name and password sign nobody in."
	tooManyTries  = "That name was given with a wrong password too often: try again after %s."
)

// nobodys is the hash a name with no password is checked against, so
// that it takes as long as one that has.
var nobodys = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(random(32)), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
})

// byPassword signs in the person a name and password are, as their key in
// an address does. Each try of a name is counted against it, whoever's it
// is, before it is checked, and cleared by a right password; while the
// name waits after wrong tries, nothing is checked.
func (s *site) byPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     *string `json:"name"`
		Password *string `json:"password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		s.refuse(w, r, err)
		return
	}
	if body.Name == nil || body.Password == nil {
		s.refuse(w, r, unreadable("The body gives a name and a password."))
		return
	}
	o := s.Keeper.Office()
	if o == nil {
		answer(w, http.StatusServiceUnavailable, "The office does not read well yet, so nobody signs in until it does.")
		return
	}
	name := strings.TrimSpace(*body.Name)
	if len(name) > 200 {
		answer(w, http.StatusForbidden, wrongPassword)
		return
	}
	until, err := tried(s.db, name, s.clock.Now())
	if err != nil {
		s.failed(w, r, err)
		return
	}
	if !until.IsZero() {
		answer(w, http.StatusTooManyRequests, fmt.Sprintf(tooManyTries, until.In(o.Zone).Format("15:04")))
		return
	}
	hash := nobodys()
	p := o.People[name]
	if p != nil && p.Password {
		own, err := passwordOf(s.db, name)
		if err != nil {
			s.failed(w, r, err)
			return
		}
		if own != nil {
			hash = own
		}
	}
	right := bcrypt.CompareHashAndPassword(hash, []byte(*body.Password)) == nil && p != nil && p.Password
	if !right {
		answer(w, http.StatusForbidden, wrongPassword)
		return
	}
	if err := triedRight(s.db, name); err != nil {
		s.failed(w, r, err)
		return
	}
	key, err := keyOf(s.db, name)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	keep(w, r, key)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"said":"You are signed in."}`)
}

// setPassword sets the one looking's password, when their entry lets
// them: a password they have already is changed only with it. Setting it
// signs out nothing.
func (s *site) setPassword(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	var body struct {
		Current  string  `json:"current"`
		Password *string `json:"password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return nil, err
	}
	if body.Password == nil {
		return nil, unreadable("The body gives the new password, and the current one when there is one.")
	}
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	if !o.People[person].Password {
		return nil, refusal("Your entry in the office's files does not let you set a password: password: true in it would, which is a change of the files, decided like any other.")
	}
	switch n := len(*body.Password); {
	case len([]rune(*body.Password)) < PasswordShortest:
		return nil, refusal(fmt.Sprintf("A password is at least %d characters.", PasswordShortest))
	case n > PasswordLongest:
		return nil, refusal(fmt.Sprintf("A password is at most %d bytes.", PasswordLongest))
	}
	own, err := passwordOf(s.db, person)
	if err != nil {
		return nil, err
	}
	if own != nil && bcrypt.CompareHashAndPassword(own, []byte(body.Current)) != nil {
		return nil, refusal("That is not your current password, so it is not changed.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(*body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`INSERT INTO server_passwords (person, hash, made) VALUES (?, ?, ?)
		ON CONFLICT(person) DO UPDATE SET hash = excluded.hash, made = excluded.made`, person, string(hash), stamp(s.clock.Now())); err != nil {
		return nil, fmt.Errorf("the password of %s cannot be kept: %w", person, err)
	}
	if own != nil {
		return map[string]string{"said": "Your password is changed."}, nil
	}
	return map[string]string{"said": "Your password is set."}, nil
}

// passwordOf gives the hash of a person's password, or nil for none.
func passwordOf(db *sql.DB, person string) ([]byte, error) {
	var hash string
	switch err := db.QueryRow(`SELECT hash FROM server_passwords WHERE person = ?`, person).Scan(&hash); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("the password of %s cannot be read: %w", person, err)
	}
	return []byte(hash), nil
}

// hasPassword says whether a person has set a password.
func hasPassword(db *sql.DB, person string) (bool, error) {
	hash, err := passwordOf(db, person)
	return hash != nil, err
}

// tried counts a try of a name before it is checked, all at once, so
// that tries sent together are counted one after another: from the
// Tries-th on, the name waits before it is tried again. While it waits, a
// try is not counted, and tried gives when it may be tried again; the
// zero time is a try counted, to be checked. The count of a name not
// tried for a day begins again.
func tried(db *sql.DB, name string, now time.Time) (time.Time, error) {
	tx, err := db.Begin()
	if err != nil {
		return time.Time{}, fmt.Errorf("a try cannot be counted: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM server_tries WHERE last < ?`, stamp(now.Add(-24*time.Hour))); err != nil {
		return time.Time{}, fmt.Errorf("the tries of a day ago cannot be let go: %w", err)
	}
	wrong, until := 0, ""
	if err := tx.QueryRow(`SELECT wrong, until FROM server_tries WHERE name = ?`, name).Scan(&wrong, &until); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, fmt.Errorf("the tries of a name cannot be read: %w", err)
	}
	if until != "" {
		t, err := time.Parse(stamped, until)
		if err != nil {
			return time.Time{}, fmt.Errorf("the tries of a name are kept with a time that cannot be read, %q: %w", until, err)
		}
		if now.Before(t) {
			return t, nil
		}
	}
	wrong++
	until = ""
	if wrong >= Tries {
		wait := TriedWait << min(wrong-Tries, 6)
		until = stamp(now.Add(min(wait, TriedLongest)))
	}
	if _, err := tx.Exec(`INSERT INTO server_tries (name, wrong, last, until) VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET wrong = excluded.wrong, last = excluded.last, until = excluded.until`, name, wrong, stamp(now), until); err != nil {
		return time.Time{}, fmt.Errorf("a try cannot be counted: %w", err)
	}
	return time.Time{}, tx.Commit()
}

// triedRight lets go of a name's wrong tries.
func triedRight(db *sql.DB, name string) error {
	if _, err := db.Exec(`DELETE FROM server_tries WHERE name = ?`, name); err != nil {
		return fmt.Errorf("the tries of a name cannot be let go: %w", err)
	}
	return nil
}
