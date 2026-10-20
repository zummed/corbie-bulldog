package changes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// A File is one file a change touches, by its path in the office: its
// whole new text, or that it goes; and the text it had in the folder when
// it was first drafted, or that it was not there.
type File struct {
	Path string `json:"path"`
	Text string `json:"text,omitzero"`
	Gone bool   `json:"gone,omitzero"`
	Was  string `json:"was,omitzero"`
	Had  bool   `json:"had,omitzero"`
}

// What a change does to a file, as list_files and the screens say it.
const (
	IsNew     = "new"
	IsChanged = "changed"
	IsGone    = "gone"
	IsAsItIs  = "as it is"
)

// Does says what the change does to the file.
func (f File) Does() string {
	switch {
	case f.Gone:
		return IsGone
	case !f.Had:
		return IsNew
	}
	return IsChanged
}

// writeFiles writes every file of a change into the office's folder, each
// whole beside itself and moved into place, and removes those that go. If
// any fails, every one already written is put back as it was, and failed
// names the file that failed. After is Bulldog's own: with failed, what
// could not be put back; without it, a folder the change left empty that
// could not be taken away, the files being written all the same.
func writeFiles(dir string, files []File) (failed, after error) {
	var done []File
	var made []string
	for _, f := range files {
		full := filepath.Join(dir, filepath.FromSlash(f.Path))
		var err error
		if f.Gone {
			err = os.Remove(full)
			if os.IsNotExist(err) {
				err = nil
			}
		} else {
			var dirs []string
			dirs, err = makeDirs(dir, filepath.Dir(full))
			made = append(made, dirs...)
			if err == nil {
				err = writeWhole(full, []byte(f.Text))
			}
		}
		if err != nil {
			return fmt.Errorf("%s could not be written, so nothing of the change was: %w", f.Path, err), errors.Join(putBack(dir, done), removeEmpty(made))
		}
		done = append(done, f)
	}
	var errs []error
	for _, f := range files {
		if f.Gone {
			errs = append(errs, removeEmpty(parents(dir, f.Path)))
		}
	}
	return nil, errors.Join(errs...)
}

// putBack puts each file of a change back as it was before it was
// written: its old text, or not there.
func putBack(dir string, files []File) error {
	var errs []error
	for _, f := range slices.Backward(files) {
		full := filepath.Join(dir, filepath.FromSlash(f.Path))
		if !f.Had {
			if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("%s could not be put back as it was: %w", f.Path, err))
			}
			errs = append(errs, removeEmpty(parents(dir, f.Path)))
			continue
		}
		if _, err := makeDirs(dir, filepath.Dir(full)); err != nil {
			errs = append(errs, fmt.Errorf("%s could not be put back as it was: %w", f.Path, err))
			continue
		}
		if err := writeWhole(full, []byte(f.Was)); err != nil {
			errs = append(errs, fmt.Errorf("%s could not be put back as it was: %w", f.Path, err))
		}
	}
	return errors.Join(errs...)
}

// writeWhole writes a file whole beside itself, under a name that starts
// with a dot and so is no file of the office, and moves it into place: a
// reader sees the old text or the new, never half of either. A file that
// is there keeps its permissions.
func writeWhole(file string, contents []byte) error {
	perm := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		perm = info.Mode().Perm()
	}
	temp, err := os.CreateTemp(filepath.Dir(file), "."+filepath.Base(file)+".*.new")
	if err != nil {
		return err
	}
	name := temp.Name()
	_, err = temp.Write(contents)
	if err == nil {
		err = temp.Sync()
	}
	if cerr := temp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(name, perm)
	}
	if err == nil {
		err = os.Rename(name, file)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}

// makeDirs makes a folder and those it is in, up to the office's own, and
// gives those it made, the deepest last.
func makeDirs(top, dir string) ([]string, error) {
	var missing []string
	for d := dir; d != top && strings.HasPrefix(d, top); d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			break
		}
		missing = append([]string{d}, missing...)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return missing, nil
}

// parents gives the folders a file of the office is in, below the
// office's own, the deepest last.
func parents(top, p string) []string {
	var dirs []string
	for d := filepath.Dir(filepath.Join(top, filepath.FromSlash(p))); d != top && strings.HasPrefix(d, top); d = filepath.Dir(d) {
		dirs = append([]string{d}, dirs...)
	}
	return dirs
}

// removeEmpty removes each of the folders that is empty, the deepest
// first. A folder that is not there is not empty.
func removeEmpty(dirs []string) error {
	var errs []error
	for _, d := range slices.Backward(dirs) {
		entries, err := os.ReadDir(d)
		switch {
		case os.IsNotExist(err) || err == nil && len(entries) > 0:
			continue
		case err != nil:
			errs = append(errs, fmt.Errorf("the folder %s, which the change may have left empty, cannot be read: %w", d, err))
			continue
		}
		if err := os.Remove(d); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("the folder %s, which the change left empty, cannot be taken away: %w", d, err))
		}
	}
	return errors.Join(errs...)
}

// Lines gives what a change does to a file by its lines: each line it
// takes away, "- ", and each it adds, "+ ", in the order of the file, the
// lines it keeps left out. A file too long to compare line by line is
// given as all its old lines taken away and all its new ones added.
func (f File) Lines() []string {
	split := func(text string, had bool) []string {
		if !had || text == "" {
			return nil
		}
		return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	was, now := split(f.Was, f.Had), split(f.Text, !f.Gone)
	var out []string
	if len(was)*len(now) > 4_000_000 {
		for _, l := range was {
			out = append(out, "- "+l)
		}
		for _, l := range now {
			out = append(out, "+ "+l)
		}
		return out
	}
	// kept[i][j] is how many lines of was[i:] and now[j:] are kept, at
	// most.
	kept := make([][]int, len(was)+1)
	for i := range kept {
		kept[i] = make([]int, len(now)+1)
	}
	for i := len(was) - 1; i >= 0; i-- {
		for j := len(now) - 1; j >= 0; j-- {
			if was[i] == now[j] {
				kept[i][j] = kept[i+1][j+1] + 1
			} else {
				kept[i][j] = max(kept[i+1][j], kept[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < len(was) || j < len(now) {
		switch {
		case i < len(was) && j < len(now) && was[i] == now[j]:
			i, j = i+1, j+1
		case j < len(now) && (i == len(was) || kept[i][j+1] > kept[i+1][j]):
			out = append(out, "+ "+now[j])
			j++
		default:
			out = append(out, "- "+was[i])
			i++
		}
	}
	return out
}
