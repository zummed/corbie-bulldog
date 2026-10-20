package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	_ "modernc.org/sqlite"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

// StoreFile is the store, in Bulldog's own folder.
const StoreFile = "bulldog.db"

// Area is the server's part's name in the store, and Version the shape of
// its tables.
const (
	Area    = "server"
	Version = 5
)

// tables makes the server's part: each person's key, and each one-time
// link sent to sign in, known by a hash of what it carries and never by
// what it carries, with the bridge and the thing it is carried as, and
// the bridges still to be tried, with whom each reached, when it was asked
// for. A link is kept only once a bridge has taken it to carry. Each
// sign-in by a method that is under way is kept by a hash of its state,
// with the browser's, by a hash of what its cookie holds, and the nonce
// and verifier sent with it. A person's password is kept as a slow hash
// of it alone, and the wrong tries of each name given with a password,
// with when it may be tried again. One row keeps the version of Bulldog
// that serves the office, and the one that ran before it.
const tables = `
CREATE TABLE server_keys (
	person TEXT PRIMARY KEY,
	key    TEXT NOT NULL UNIQUE,
	made   TEXT NOT NULL
);
CREATE TABLE server_links (
	hash   TEXT PRIMARY KEY,
	person TEXT NOT NULL,
	made   TEXT NOT NULL,
	used   TEXT NOT NULL DEFAULT '',
	bridge TEXT NOT NULL,
	thing  INTEGER NOT NULL,
	rest   TEXT NOT NULL
);
CREATE INDEX server_links_person ON server_links (person, made);
CREATE INDEX server_links_thing ON server_links (thing);
CREATE TABLE server_signing (
	state    TEXT PRIMARY KEY,
	method   TEXT NOT NULL,
	browser  TEXT NOT NULL,
	nonce    TEXT NOT NULL,
	verifier TEXT NOT NULL,
	made     TEXT NOT NULL
);
CREATE TABLE server_passwords (
	person TEXT PRIMARY KEY,
	hash   TEXT NOT NULL,
	made   TEXT NOT NULL
);
CREATE TABLE server_tries (
	name   TEXT PRIMARY KEY,
	wrong  INTEGER NOT NULL,
	last   TEXT NOT NULL,
	until  TEXT NOT NULL
);
CREATE TABLE server_versions (
	one     INTEGER PRIMARY KEY CHECK (one = 1),
	running TEXT NOT NULL,
	before  TEXT NOT NULL
);
`

// stamped is how a time is kept in the server's part: in UTC, and always
// as wide, so that times kept as text sort as times do.
const stamped = "2006-01-02T15:04:05.000000000Z"

func stamp(t time.Time) string { return t.UTC().Format(stamped) }

// OpenStore opens the office's store, one file that every part of Bulldog
// keeps its own part of, and readies the server's part. Each write to it
// takes the store whole from its first statement, so a part that reads
// and then writes is never refused by another's write in between.
func OpenStore(dir string) (*sql.DB, error) {
	file := filepath.Join(dir, shipped.Dir, StoreFile)
	db, err := sql.Open("sqlite", file+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("the store, %s, cannot be opened: %w", file, err)
	}
	if err := ready(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// ready makes the server's tables in a store that has none, and refuses a
// part made by another Bulldog.
func ready(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS parts (area TEXT PRIMARY KEY, version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("the store cannot be read: %w", err)
	}
	var version int
	switch err := db.QueryRow(`SELECT version FROM parts WHERE area = ?`, Area).Scan(&version); {
	case errors.Is(err, sql.ErrNoRows):
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.Exec(tables); err != nil {
			return fmt.Errorf("the server's tables cannot be made in the store: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO parts (area, version) VALUES (?, ?)`, Area, Version); err != nil {
			return err
		}
		return tx.Commit()
	case err != nil:
		return fmt.Errorf("the store cannot be read: %w", err)
	case version != Version:
		return fmt.Errorf("the store's keys were kept by another Bulldog, in the shape of version %d and not %d: remove .bulldog/bulldog.db and start again, and the office starts again with nothing kept", version, Version)
	}
	return nil
}

// ran keeps the version of Bulldog that serves the office now, and gives
// the one that ran before it, "" when none did, and whether this start is
// the first of the version. A start of the same version again gives the
// same before, so what changed since it is shown while the version runs.
func ran(db *sql.DB, version string) (before string, upgraded bool, err error) {
	tx, err := db.Begin()
	if err != nil {
		return "", false, fmt.Errorf("the version that ran before cannot be read: %w", err)
	}
	defer tx.Rollback()
	var running string
	switch err := tx.QueryRow(`SELECT running, before FROM server_versions`).Scan(&running, &before); {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.Exec(`INSERT INTO server_versions (one, running, before) VALUES (1, ?, '')`, version)
		if err != nil {
			return "", false, fmt.Errorf("the version cannot be kept: %w", err)
		}
		return "", false, tx.Commit()
	case err != nil:
		return "", false, fmt.Errorf("the version that ran before cannot be read: %w", err)
	case running == version:
		return before, false, nil
	}
	if _, err := tx.Exec(`UPDATE server_versions SET running = ?, before = ?`, version, running); err != nil {
		return "", false, fmt.Errorf("the version cannot be kept: %w", err)
	}
	return running, true, tx.Commit()
}

// keyed gives every person of a good reading a key, making one for each
// person new in it, and destroys the key of each person no longer in it,
// so that someone written later under the same name is given a new one;
// and destroys the password of each who left it or whose entry no longer
// says password: true. It gives the people newly given a key, in the
// office's order, those whose key it destroyed, and those whose password
// it destroyed while they stay.
func keyed(db *sql.DB, o *office.Office, now time.Time) (made, gone, unset []string, err error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("the people's keys cannot be read: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT person FROM server_keys`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("the people's keys cannot be read: %w", err)
	}
	var had []string
	for rows.Next() {
		var person string
		if err := rows.Scan(&person); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		had = append(had, person)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	for _, person := range had {
		if _, ok := o.People[person]; ok {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM server_keys WHERE person = ?`, person); err != nil {
			return nil, nil, nil, fmt.Errorf("the key of %s, who is no longer in the office, cannot be destroyed: %w", person, err)
		}
		if _, err := tx.Exec(`DELETE FROM server_links WHERE person = ?`, person); err != nil {
			return nil, nil, nil, fmt.Errorf("the sign-in links of %s, who is no longer in the office, cannot be destroyed: %w", person, err)
		}
		gone = append(gone, person)
	}
	for _, person := range o.Everyone {
		if slices.Contains(had, person) {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO server_keys (person, key, made) VALUES (?, ?, ?)`, person, random(32), stamp(now)); err != nil {
			return nil, nil, nil, fmt.Errorf("a key cannot be made for %s: %w", person, err)
		}
		made = append(made, person)
	}
	if unset, err = unsetPasswords(tx, o); err != nil {
		return nil, nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, nil, fmt.Errorf("the people's keys cannot be kept: %w", err)
	}
	return made, gone, unset, nil
}

// unsetPasswords destroys the password of each person no longer in the
// office, or whose entry no longer lets them set one, and gives those who
// are still in it.
func unsetPasswords(tx *sql.Tx, o *office.Office) ([]string, error) {
	rows, err := tx.Query(`SELECT person FROM server_passwords ORDER BY person`)
	if err != nil {
		return nil, fmt.Errorf("the people's passwords cannot be read: %w", err)
	}
	var had []string
	for rows.Next() {
		var person string
		if err := rows.Scan(&person); err != nil {
			rows.Close()
			return nil, err
		}
		had = append(had, person)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var unset []string
	for _, person := range had {
		p := o.People[person]
		if p != nil && p.Password {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM server_passwords WHERE person = ?`, person); err != nil {
			return nil, fmt.Errorf("the password of %s cannot be destroyed: %w", person, err)
		}
		if p != nil {
			unset = append(unset, person)
		}
	}
	return unset, nil
}

// keyOf gives a person's key.
func keyOf(db *sql.DB, person string) (string, error) {
	var key string
	if err := db.QueryRow(`SELECT key FROM server_keys WHERE person = ?`, person).Scan(&key); err != nil {
		return "", fmt.Errorf("the key of %s cannot be read: %w", person, err)
	}
	return key, nil
}

// whoseKey gives the person whose key a browser showed, or "" for a key
// that is nobody's. Every key is compared whole, each in the same time,
// so how long it takes says nothing of how near a guess came.
func whoseKey(db *sql.DB, shown string) (string, error) {
	rows, err := db.Query(`SELECT person, key FROM server_keys`)
	if err != nil {
		return "", fmt.Errorf("the people's keys cannot be read: %w", err)
	}
	defer rows.Close()
	whose := ""
	for rows.Next() {
		var person, key string
		if err := rows.Scan(&person, &key); err != nil {
			return "", fmt.Errorf("the people's keys cannot be read: %w", err)
		}
		if subtle.ConstantTimeCompare([]byte(key), []byte(shown)) == 1 {
			whose = person
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("the people's keys cannot be read: %w", err)
	}
	return whose, nil
}

// newKey gives a person a key in place of the one they had, and gives it.
func newKey(db *sql.DB, person string, now time.Time) (string, error) {
	key := random(32)
	res, err := db.Exec(`UPDATE server_keys SET key = ?, made = ? WHERE person = ?`, key, stamp(now), person)
	if err != nil {
		return "", fmt.Errorf("a new key cannot be made for %s: %w", person, err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return "", fmt.Errorf("a new key cannot be made for %s, who has none", person)
	}
	return key, nil
}

// hashed gives what a one-time link is known by in the store.
func hashed(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// A link is a one-time sign-in link as the store keeps it: whose it is,
// when it was asked for, whether it is used up, the bridge and the thing
// it is carried as, and the bridges still to be tried, in order, with whom
// each reached when it was asked for.
type link struct {
	hash, person string
	made         time.Time
	used         bool
	bridge       string
	thing        int64
	rest         []office.Identity
}

const linkColumns = `SELECT hash, person, made, used, bridge, thing, rest FROM server_links`

func scanLink(row *sql.Row) (*link, error) {
	var l link
	var made, used, rest string
	switch err := row.Scan(&l.hash, &l.person, &made, &used, &l.bridge, &l.thing, &rest); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("the sign-in links cannot be read: %w", err)
	}
	t, err := time.Parse(stamped, made)
	if err != nil {
		return nil, fmt.Errorf("a sign-in link is kept with a time that cannot be read, %q: %w", made, err)
	}
	if err := json.Unmarshal([]byte(rest), &l.rest); err != nil {
		return nil, fmt.Errorf("a sign-in link is kept with bridges to try that cannot be read: %w", err)
	}
	l.made, l.used = t, used != ""
	return &l, nil
}

// lastLink gives when the latest link was asked for for a person, or the
// zero time.
func lastLink(db *sql.DB, person string) (time.Time, error) {
	l, err := scanLink(db.QueryRow(linkColumns+` WHERE person = ? ORDER BY made DESC LIMIT 1`, person))
	if err != nil || l == nil {
		return time.Time{}, err
	}
	return l.made, nil
}

// keepLink keeps a link a bridge has taken to carry, by the hash of what
// it carries.
func keepLink(db *sql.DB, token string, l link) error {
	rest, err := json.Marshal(orEmpty(l.rest))
	if err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO server_links (hash, person, made, bridge, thing, rest) VALUES (?, ?, ?, ?, ?, ?)`,
		hashed(token), l.person, stamp(l.made), l.bridge, l.thing, string(rest)); err != nil {
		return fmt.Errorf("the one-time link of %s, which %s is carrying, cannot be kept, so it signs nobody in: %w", l.person, l.bridge, err)
	}
	return nil
}

// linkOf gives the link being carried as a thing, or nil.
func linkOf(db *sql.DB, thing int64) (*link, error) {
	return scanLink(db.QueryRow(linkColumns+` WHERE thing = ?`, thing))
}

// spent uses up a link that was not sent, so that what it carries, which
// reached nobody, signs nobody in.
func spent(db *sql.DB, hash string, now time.Time) error {
	if _, err := db.Exec(`UPDATE server_links SET used = ? WHERE hash = ?`, stamp(now), hash); err != nil {
		return fmt.Errorf("a one-time link that was not sent cannot be used up: %w", err)
	}
	return nil
}

// useLink uses up a link, and gives whose it was: a link that is no
// link, used already, or older than Linked, signs nobody in, and gives "".
func useLink(db *sql.DB, token string, now time.Time) (string, error) {
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("the sign-in links cannot be read: %w", err)
	}
	defer tx.Rollback()
	l, err := scanLink(tx.QueryRow(linkColumns+` WHERE hash = ?`, hashed(token)))
	if err != nil || l == nil || l.used || now.Sub(l.made) > Linked {
		return "", err
	}
	if _, err := tx.Exec(`UPDATE server_links SET used = ? WHERE hash = ?`, stamp(now), l.hash); err != nil {
		return "", fmt.Errorf("a sign-in link cannot be used up: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("a sign-in link cannot be used up: %w", err)
	}
	return l.person, nil
}
