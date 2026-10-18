package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

// IdentityFile is the file in Bulldog's own folder that holds the office's
// identity and the folder it was made in.
const IdentityFile = "identity"

// Folder gives the folder a command is about, as a whole path: the one
// named, or, with none named, the folder it is run in when that holds an
// office.yaml, and otherwise bulldog in the person's home.
func Folder(named string) (string, error) {
	if named != "" {
		return filepath.Abs(named)
	}
	here, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(here, office.OfficeFile)); err == nil {
		return here, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no folder is named, and the one this is run in holds no %s, so the office is bulldog in your home; but where your home is cannot be told: %w", office.OfficeFile, err)
	}
	return filepath.Join(home, "bulldog"), nil
}

// A Readying says how a folder is readied. Make makes a folder that is not
// there, and Move takes an office whose own folder was made in another as
// that office, moved here.
type Readying struct {
	Make bool
	Move bool
}

// An Identity is the office's identity, by which every run of it is
// labelled and found, and the folder, by its real path, it was made in.
type Identity struct {
	ID     string `yaml:"identity"`
	Folder string `yaml:"folder"`
}

// Ready readies a folder to be served, saying each thing it does: a folder
// with nothing in it is given the example office and made a repository of
// its own; an office in no repository is made one, with what is there as
// its first commit; an office with no folder of Bulldog's own is given
// one, with the office's identity; and the files Bulldog ships are written
// there, whole. A folder that is not there and is not to be made, one that
// holds things and no office.yaml, and an office whose own folder was made
// in another, unless that is to be taken as a move, are refused, and
// nothing is written.
func Ready(dir string, how Readying, now time.Time, say func(format string, args ...any)) (Identity, error) {
	in, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist) && !how.Make:
		return Identity{}, fmt.Errorf("%s is not there, and nothing was made: corbie bulldog init %s makes an office there", dir, dir)
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Identity{}, err
		}
	case err != nil:
		return Identity{}, err
	}
	fresh := len(in) == 0
	if fresh {
		if err := WriteFolder(dir, examples.NewHouse()); err != nil {
			return Identity{}, fmt.Errorf("the example office cannot be written into %s: %w", dir, err)
		}
		say("there was no office in %s, so the example was written there. It is yours to change: start with %s", dir, office.OfficeFile)
		if _, err := changes.BeginOwn(dir, now); err != nil {
			return Identity{}, err
		}
		say("%s is a git repository now, and what is there is its first commit", dir)
	} else if _, err := os.Stat(filepath.Join(dir, office.OfficeFile)); errors.Is(err, fs.ErrNotExist) {
		return Identity{}, fmt.Errorf("%s has things in it and no %s, so it is no office, and it was left as it was: an office is begun in a folder that is empty or not there yet", dir, office.OfficeFile)
	} else if err != nil {
		return Identity{}, err
	}
	id, err := identityOf(dir, how, fresh, say)
	if err != nil {
		return Identity{}, err
	}
	if !fresh {
		switch _, err := changes.Begin(dir, now); {
		case err == nil:
			say("%s is a git repository now, and what is there is its first commit", dir)
		case !errors.Is(err, changes.ErrInRepository):
			return Identity{}, err
		}
	}
	if err := shipped.Write(dir); err != nil {
		return Identity{}, fmt.Errorf("the files Bulldog ships cannot be written into %s: %w", filepath.Join(dir, shipped.Dir), err)
	}
	return id, nil
}

// WriteFolder writes files into a folder, each under its path there.
func WriteFolder(dir string, files map[string][]byte) error {
	for name, contents := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, contents, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// identityOf gives the office's identity, making it the first time, and
// refuses one made in another folder unless it is to be taken as a move.
func identityOf(dir string, how Readying, fresh bool, say func(string, ...any)) (Identity, error) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return Identity{}, err
	}
	id, err := ReadIdentity(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		id = Identity{ID: random(16), Folder: real}
		if err := writeIdentity(dir, id); err != nil {
			return Identity{}, err
		}
		if !fresh {
			say("%s has a folder of Bulldog's own now, %s, with the office's identity", dir, shipped.Dir+"/")
		}
		return id, nil
	case err != nil:
		return Identity{}, err
	case id.Folder == real:
		return id, nil
	case !how.Move:
		return Identity{}, fmt.Errorf("%s was made in %s, not here, so this is a copy of that office, or the office was moved: for an office of its own, remove %s; if the office was moved here, run corbie bulldog init %s",
			filepath.Join(dir, shipped.Dir)+"/", id.Folder, filepath.Join(dir, shipped.Dir)+"/", dir)
	}
	was := id.Folder
	id.Folder = real
	if err := writeIdentity(dir, id); err != nil {
		return Identity{}, err
	}
	say("%s was made in %s, and is taken as that office moved here: its identity, its store and its runs are kept", dir, was)
	return id, nil
}

// ReadIdentity reads the office's identity from its own folder.
func ReadIdentity(dir string) (Identity, error) {
	file := filepath.Join(dir, shipped.Dir, IdentityFile)
	contents, err := os.ReadFile(file)
	if err != nil {
		return Identity{}, err
	}
	var id Identity
	if err := yaml.Unmarshal(contents, &id); err != nil || id.ID == "" || id.Folder == "" {
		return Identity{}, fmt.Errorf("%s cannot be read as the office's identity and the folder it was made in: remove %s to make the office again, with nothing kept", file, filepath.Join(dir, shipped.Dir)+"/")
	}
	return id, nil
}

// writeIdentity writes the office's identity into its own folder.
func writeIdentity(dir string, id Identity) error {
	if err := os.MkdirAll(filepath.Join(dir, shipped.Dir), 0o755); err != nil {
		return err
	}
	text := fmt.Sprintf("# The office's identity, by which every run of it is labelled and found in\n# Corbie, and the folder it was made in.\nidentity: %s\nfolder: %q\n", id.ID, id.Folder)
	file := filepath.Join(dir, shipped.Dir, IdentityFile)
	temp := file + ".new"
	if err := os.WriteFile(temp, []byte(text), 0o644); err != nil {
		return err
	}
	return os.Rename(temp, file)
}

// random gives so many bytes no one could guess, written in hex.
func random(size int) string {
	b := make([]byte, size)
	rand.Read(b)
	return hex.EncodeToString(b)
}
