package changes

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Placeholder is the address every commit Bulldog makes gives its author
// and committer, whoever they are: the history holds no address of anyone.
const Placeholder = "office@bulldog.invalid"

// The notes a commit ends with.
const (
	DecidedBy = "Decided-by"
	NoteKey   = "Note"
	RequestOf = "Request"
)

// The headings of what a commit says the change does.
const (
	doesHeading = "What it does:"
	moreHeading = "What it lets more by:"
)

// A repo is the git repository an office's folder is in, read and written
// with a library: no git program is run, and nothing is pushed or pulled.
type repo struct {
	git *git.Repository
	// root is the repository's own folder on disk.
	root string
	// in is the office's folder within it, "" when the folder is the
	// repository's own.
	in string
}

// ErrNoRepository says an office's folder is in no git repository.
var ErrNoRepository = errors.New("the office's folder is in no git repository, so no change can be kept: the server makes one when it starts")

// ErrInRepository says an office's folder is in a git repository already.
var ErrInRepository = errors.New("the office's folder is in a git repository already")

// openRepo opens the repository an office's folder is in.
func openRepo(dir string) (*repo, error) {
	g, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, ErrNoRepository
	}
	if err != nil {
		return nil, fmt.Errorf("the office's repository cannot be opened: %w", err)
	}
	wt, err := g.Worktree()
	if err != nil {
		return nil, fmt.Errorf("the office's repository has no folder of files: %w", err)
	}
	root, err := filepath.EvalSymlinks(wt.Filesystem.Root())
	if err != nil {
		return nil, err
	}
	full, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	in, err := filepath.Rel(root, full)
	if err != nil {
		return nil, err
	}
	if in = filepath.ToSlash(in); in == "." {
		in = ""
	}
	return &repo{git: g, root: root, in: in}, nil
}

// Begin makes an office's folder that is in no repository a repository of
// its own, and commits every file of the office there as its first commit,
// by Bulldog. What starts with a dot is left out, and Bulldog's own folder
// is excluded from the repository. A folder that is a repository already,
// or is within one, is refused: its history is that repository's.
func Begin(dir string, now time.Time) (string, error) {
	switch r, err := openRepo(dir); {
	case err == nil:
		return "", fmt.Errorf("%s is in the git repository at %s already, and its history is that one's: %w", dir, r.root, ErrInRepository)
	case !errors.Is(err, ErrNoRepository):
		return "", err
	}
	return begin(dir, now)
}

// BeginOwn makes the folder of an office Bulldog has only now written a
// repository of its own, even within another, and commits every file of
// it as its first commit, as Begin does. A folder that is a repository's
// own already is refused.
func BeginOwn(dir string, now time.Time) (string, error) {
	switch _, err := os.Stat(filepath.Join(dir, ".git")); {
	case err == nil:
		return "", fmt.Errorf("%s is a git repository already, and its history is that one's", dir)
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	return begin(dir, now)
}

// begin makes a folder a repository, Bulldog's own folder excluded, that
// takes a person's push onto the branch it has out, writing the files with
// it, and commits every file of the office there.
func begin(dir string, now time.Time) (string, error) {
	made, err := git.PlainInit(dir, false)
	if err != nil {
		return "", fmt.Errorf("%s cannot be made a repository: %w", dir, err)
	}
	cfg, err := made.Config()
	if err != nil {
		return "", err
	}
	cfg.Raw.Section("receive").SetOption("denyCurrentBranch", "updateInstead")
	if err := made.SetConfig(cfg); err != nil {
		return "", fmt.Errorf("%s cannot be set to take a push: %w", dir, err)
	}
	exclude := filepath.Join(dir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(exclude, []byte("# Bulldog's own: its store, its identity and the files it ships.\n/.bulldog/\n"), 0o644); err != nil {
		return "", err
	}
	r, err := openRepo(dir)
	if err != nil {
		return "", err
	}
	files, err := onDisk(dir)
	if err != nil {
		return "", err
	}
	return r.commit(files, "bulldog", "The office as it was when its history began\n", now)
}

// onDisk gives every file of the office in a folder, by its path in it:
// nothing whose name starts with a dot.
func onDisk(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(file string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file == dir {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, file)
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	slices.Sort(files)
	return files, err
}

// full gives a path of the office as a path of the repository.
func (r *repo) full(p string) string { return path.Join(r.in, p) }

// head gives the last commit, or nil while there is none.
func (r *repo) head() (*object.Commit, error) {
	ref, err := r.git.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.git.CommitObject(ref.Hash())
}

// committed gives a file of the office as the last commit holds it, and
// whether it holds it.
func (r *repo) committed(head *object.Commit, p string) (blob []byte, mode filemode.FileMode, ok bool, err error) {
	if head == nil {
		return nil, 0, false, nil
	}
	tree, err := head.Tree()
	if err != nil {
		return nil, 0, false, err
	}
	entry, err := tree.FindEntry(r.full(p))
	if errors.Is(err, object.ErrEntryNotFound) || errors.Is(err, object.ErrDirectoryNotFound) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	if entry.Mode == filemode.Dir || entry.Mode == filemode.Submodule {
		return nil, entry.Mode, false, nil
	}
	contents, err := r.blob(entry.Hash)
	return contents, entry.Mode, err == nil, err
}

func (r *repo) blob(h plumbing.Hash) ([]byte, error) {
	b, err := r.git.BlobObject(h)
	if err != nil {
		return nil, err
	}
	rd, err := b.Reader()
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	return io.ReadAll(rd)
}

// disk gives a file of the office as it is on disk, as git would keep it:
// a link as where it points. It says whether it is there.
func (r *repo) disk(p string) ([]byte, bool, error) {
	file := filepath.Join(r.root, filepath.FromSlash(r.full(p)))
	info, err := os.Lstat(file)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(file)
		return []byte(target), true, err
	case !info.Mode().IsRegular():
		return nil, false, nil
	}
	contents, err := os.ReadFile(file)
	return contents, true, err
}

// edited gives those of the files whose text on disk is not as the last
// commit holds it: what someone changed by hand and has not committed.
func (r *repo) edited(files []string) ([]string, error) {
	head, err := r.head()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range files {
		was, _, had, err := r.committed(head, p)
		if err != nil {
			return nil, err
		}
		now, has, err := r.disk(p)
		if err != nil {
			return nil, err
		}
		if had != has || !bytes.Equal(was, now) {
			out = append(out, p)
		}
	}
	return out, nil
}

// byHand gives every file of the office that is not as the last commit
// left it, read from the folder now: those changed, added or removed by
// hand.
func (r *repo) byHand(dir string) ([]string, error) {
	files, err := onDisk(dir)
	if err != nil {
		return nil, err
	}
	head, err := r.head()
	if err != nil {
		return nil, err
	}
	if head != nil {
		tree, err := head.Tree()
		if err != nil {
			return nil, err
		}
		if r.in != "" {
			tree, err = tree.Tree(r.in)
			if errors.Is(err, object.ErrDirectoryNotFound) {
				tree = nil
			} else if err != nil {
				return nil, err
			}
		}
		if tree != nil {
			err := tree.Files().ForEach(func(f *object.File) error {
				if !hasDot(f.Name) && !slices.Contains(files, f.Name) {
					files = append(files, f.Name)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	slices.Sort(files)
	return r.edited(files)
}

// commit keeps the files of the office, by their paths in it, as they are
// on disk now, in a commit made from the last commit and exactly those
// files, whatever else is staged; a file that is gone from disk is gone
// from the commit. The index is set to hold them as committed, so they are
// not shown as changed, and is otherwise left as it was. It gives the
// commit's id once its branch names it, even with an error: the commit is
// made, and the error is the index that could not be written.
func (r *repo) commit(files []string, by, message string, now time.Time) (string, error) {
	head, err := r.head()
	if err != nil {
		return "", err
	}
	var tree *object.Tree
	if head != nil {
		if tree, err = head.Tree(); err != nil {
			return "", err
		}
	}
	type put struct {
		blob plumbing.Hash
		mode filemode.FileMode
		size int
		gone bool
	}
	puts := map[string]put{}
	for _, p := range files {
		contents, has, err := r.disk(p)
		if err != nil {
			return "", err
		}
		if !has {
			puts[r.full(p)] = put{gone: true}
			continue
		}
		mode := filemode.Regular
		info, err := os.Lstat(filepath.Join(r.root, filepath.FromSlash(r.full(p))))
		if err != nil {
			return "", err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			mode = filemode.Symlink
		case info.Mode()&0o111 != 0:
			mode = filemode.Executable
		}
		h, err := r.store(plumbing.BlobObject, contents)
		if err != nil {
			return "", err
		}
		puts[r.full(p)] = put{blob: h, mode: mode, size: len(contents)}
	}
	// Each tree from the top, with what is put in it.
	var build func(t *object.Tree, at string) (plumbing.Hash, bool, error)
	build = func(t *object.Tree, at string) (plumbing.Hash, bool, error) {
		entries := map[string]object.TreeEntry{}
		if t != nil {
			for _, e := range t.Entries {
				entries[e.Name] = e
			}
		}
		// The names directly in this tree that something put is under.
		under := map[string]bool{}
		for full, p := range puts {
			rest, ok := strings.CutPrefix(full, at)
			if !ok {
				continue
			}
			name, _, isDir := strings.Cut(rest, "/")
			if !isDir {
				if p.gone {
					delete(entries, name)
				} else {
					entries[name] = object.TreeEntry{Name: name, Mode: p.mode, Hash: p.blob}
				}
				continue
			}
			under[name] = true
		}
		for name := range under {
			var sub *object.Tree
			if e, ok := entries[name]; ok && e.Mode == filemode.Dir {
				s, err := r.git.TreeObject(e.Hash)
				if err != nil {
					return plumbing.ZeroHash, false, err
				}
				sub = s
			}
			h, empty, err := build(sub, at+name+"/")
			if err != nil {
				return plumbing.ZeroHash, false, err
			}
			if empty {
				delete(entries, name)
			} else {
				entries[name] = object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: h}
			}
		}
		if len(entries) == 0 && at != "" {
			return plumbing.ZeroHash, true, nil
		}
		out := &object.Tree{}
		for _, e := range entries {
			out.Entries = append(out.Entries, e)
		}
		// Git sorts a tree's entries by name, a folder's as if it ended in
		// a slash.
		key := func(e object.TreeEntry) string {
			if e.Mode == filemode.Dir {
				return e.Name + "/"
			}
			return e.Name
		}
		slices.SortFunc(out.Entries, func(a, b object.TreeEntry) int { return strings.Compare(key(a), key(b)) })
		obj := r.git.Storer.NewEncodedObject()
		if err := out.Encode(obj); err != nil {
			return plumbing.ZeroHash, false, err
		}
		h, err := r.git.Storer.SetEncodedObject(obj)
		return h, false, err
	}
	top, _, err := build(tree, "")
	if err != nil {
		return "", err
	}
	who := object.Signature{Name: by, Email: Placeholder, When: now}
	c := &object.Commit{Author: who, Committer: who, Message: message, TreeHash: top}
	if head != nil {
		c.ParentHashes = []plumbing.Hash{head.Hash}
	}
	obj := r.git.Storer.NewEncodedObject()
	if err := c.Encode(obj); err != nil {
		return "", err
	}
	id, err := r.git.Storer.SetEncodedObject(obj)
	if err != nil {
		return "", err
	}
	// The index holds each file as it is now committed; the rest of what
	// it holds, staged by hand or not, is left as it was.
	idx, err := r.git.Storer.Index()
	if err != nil {
		return "", err
	}
	for full, p := range puts {
		if p.gone {
			idx.Remove(full)
			continue
		}
		e, err := idx.Entry(full)
		if err != nil {
			e = idx.Add(full)
		}
		e.Hash, e.Mode, e.Size = p.blob, p.mode, uint32(p.size)
		if info, err := os.Lstat(filepath.Join(r.root, filepath.FromSlash(full))); err == nil {
			e.ModifiedAt, e.CreatedAt = info.ModTime(), info.ModTime()
		}
	}
	idx.Cache = nil
	// The branch moves on before the index is written: a commit is made
	// once its branch names it.
	if err := r.advance(id); err != nil {
		return "", err
	}
	if err := r.git.Storer.SetIndex(idx); err != nil {
		return id.String(), fmt.Errorf("the commit %s is made, but the repository's index cannot be written, so git may show its files as changed until it is: %w", id, err)
	}
	return id.String(), nil
}

// store writes an object, and gives its id.
func (r *repo) store(t plumbing.ObjectType, contents []byte) (plumbing.Hash, error) {
	obj := r.git.Storer.NewEncodedObject()
	obj.SetType(t)
	w, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if _, err := w.Write(contents); err != nil {
		return plumbing.ZeroHash, err
	}
	if err := w.Close(); err != nil {
		return plumbing.ZeroHash, err
	}
	return r.git.Storer.SetEncodedObject(obj)
}

// advance moves the branch HEAD is on to a commit, making it when it is
// not yet there.
func (r *repo) advance(id plumbing.Hash) error {
	head, err := r.git.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}
	name := plumbing.HEAD
	if head.Type() == plumbing.SymbolicReference {
		name = head.Target()
	}
	return r.git.Storer.SetReference(plumbing.NewHashReference(name, id))
}

// message writes a commit's message: the first line of why, the rest of
// why, what the change does with what lets more by apart, and its notes
// last, each a key, a colon and a value on one line.
func message(why string, does, more []string, notes [][2]string) string {
	why = strings.TrimSpace(strings.ReplaceAll(why, "\r\n", "\n"))
	subject, rest, _ := strings.Cut(why, "\n")
	var b strings.Builder
	b.WriteString(strings.TrimSpace(subject) + "\n")
	if rest = strings.TrimSpace(rest); rest != "" {
		b.WriteString("\n" + rest + "\n")
	}
	for _, part := range []struct {
		heading string
		lines   []string
	}{{doesHeading, does}, {moreHeading, more}} {
		if len(part.lines) == 0 {
			continue
		}
		b.WriteString("\n" + part.heading + "\n")
		for _, line := range part.lines {
			b.WriteString("- " + oneLine(line) + "\n")
		}
	}
	var kept []string
	for _, n := range notes {
		if value := oneLine(n[1]); value != "" {
			kept = append(kept, n[0]+": "+value)
		}
	}
	if len(kept) > 0 {
		b.WriteString("\n" + strings.Join(kept, "\n") + "\n")
	}
	return b.String()
}

// oneLine gives a text on one line: each line ending is a space.
func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\r", " ")), " ")
}

// A Commit is one commit that touched the office's files: a change, a
// note, or the office's first commit.
type Commit struct {
	ID string    `json:"id"`
	At time.Time `json:"at"`
	By string    `json:"by"`
	// Why is the first line, and More the rest of why.
	Why  string `json:"why"`
	More string `json:"more,omitzero"`
	// Does is what the change does, with what lets more by apart.
	Does       []string `json:"does,omitzero"`
	LetsMoreBy []string `json:"lets_more_by,omitzero"`
	// DecidedBy is who decided, nobody for a note; Note is their note.
	DecidedBy string `json:"decided_by,omitzero"`
	Note      string `json:"note,omitzero"`
	Request   int64  `json:"request,omitzero"`
	// Notes are every note it ends with, by key.
	Notes map[string]string `json:"notes,omitzero"`
	// Files are the office's files it touched.
	Files []string `json:"files"`
}

// A Version is one file as a commit left it, beside what it was before.
type Version struct {
	Path string `json:"path"`
	Was  string `json:"was"`
	Now  string `json:"now"`
	New  bool   `json:"new,omitzero"`
	Gone bool   `json:"gone,omitzero"`
	// Binary says the file is not text, and is not shown.
	Binary bool `json:"binary,omitzero"`
}

// read reads a commit's message back.
func read(c *object.Commit, files []string) Commit {
	out := Commit{ID: c.Hash.String(), At: c.Author.When, By: c.Author.Name, Notes: map[string]string{}, Files: files}
	lines := strings.Split(strings.TrimRight(c.Message, "\n"), "\n")
	out.Why = lines[0]
	end := len(lines)
	for end > 1 {
		key, value, ok := strings.Cut(lines[end-1], ": ")
		if !ok || key == "" || strings.ContainsAny(key, " \t") || strings.HasPrefix(key, "-") {
			break
		}
		out.Notes[key] = value
		end--
	}
	out.DecidedBy, out.Note = out.Notes[DecidedBy], out.Notes[NoteKey]
	out.Request, _ = strconv.ParseInt(out.Notes[RequestOf], 10, 64)
	var more []string
	var into *[]string
	for _, line := range lines[1:end] {
		switch {
		case line == doesHeading:
			into = &out.Does
		case line == moreHeading:
			into = &out.LetsMoreBy
		case into != nil && strings.HasPrefix(line, "- "):
			*into = append(*into, strings.TrimPrefix(line, "- "))
		case into != nil && line == "":
		default:
			into = nil
			more = append(more, line)
		}
	}
	out.More = strings.TrimSpace(strings.Join(more, "\n"))
	return out
}

// subtree gives the office's tree within a commit, or nil where it has
// none.
func (r *repo) subtree(c *object.Commit) (*object.Tree, error) {
	tree, err := c.Tree()
	if err != nil || r.in == "" {
		return tree, err
	}
	tree, err = tree.Tree(r.in)
	if errors.Is(err, object.ErrDirectoryNotFound) {
		return nil, nil
	}
	return tree, err
}

// changed gives the office's files a commit changed, each with what it was
// and what it is, nil where it was not there or is gone.
func (r *repo) changed(c *object.Commit) (map[string][2]*object.File, error) {
	now, err := r.subtree(c)
	if err != nil {
		return nil, err
	}
	var before *object.Tree
	if c.NumParents() > 0 {
		parent, err := c.Parent(0)
		if err != nil {
			return nil, err
		}
		if before, err = r.subtree(parent); err != nil {
			return nil, err
		}
	}
	if before == nil {
		before = &object.Tree{}
	}
	if now == nil {
		now = &object.Tree{}
	}
	diffs, err := before.Diff(now)
	if err != nil {
		return nil, err
	}
	out := map[string][2]*object.File{}
	for _, d := range diffs {
		was, is, err := d.Files()
		if err != nil {
			return nil, err
		}
		for _, name := range []string{d.From.Name, d.To.Name} {
			if name == "" || hasDot(name) {
				continue
			}
			pair := out[name]
			if name == d.From.Name {
				pair[0] = was
			}
			if name == d.To.Name {
				pair[1] = is
			}
			out[name] = pair
		}
	}
	return out, nil
}

// log gives the commits that touched the office's files, newest first.
func (r *repo) log() ([]Commit, error) {
	head, err := r.head()
	if err != nil || head == nil {
		return []Commit{}, err
	}
	commits, err := r.git.Log(&git.LogOptions{From: head.Hash})
	if err != nil {
		return nil, err
	}
	defer commits.Close()
	out := []Commit{}
	err = commits.ForEach(func(c *object.Commit) error {
		files, err := r.changed(c)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		var names []string
		for name := range files {
			names = append(names, name)
		}
		slices.Sort(names)
		out = append(out, read(c, names))
		return nil
	})
	return out, err
}

// show gives one commit, with each of the office's files it touched as it
// was and as it is.
func (r *repo) show(id string) (Commit, []Version, error) {
	if !plumbing.IsHash(id) {
		return Commit{}, nil, ErrNotThere
	}
	c, err := r.git.CommitObject(plumbing.NewHash(id))
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return Commit{}, nil, ErrNotThere
	}
	if err != nil {
		return Commit{}, nil, fmt.Errorf("the change %s cannot be read: %w", id, err)
	}
	head, err := r.head()
	if err != nil {
		return Commit{}, nil, err
	}
	if head == nil {
		return Commit{}, nil, ErrNotThere
	}
	if c.Hash != head.Hash {
		if before, err := c.IsAncestor(head); err != nil {
			return Commit{}, nil, fmt.Errorf("the change %s cannot be found in the history: %w", id, err)
		} else if !before {
			return Commit{}, nil, ErrNotThere
		}
	}
	files, err := r.changed(c)
	if err != nil {
		return Commit{}, nil, err
	}
	if len(files) == 0 {
		return Commit{}, nil, ErrNotThere
	}
	var names []string
	var versions []Version
	for name, pair := range files {
		names = append(names, name)
		v := Version{Path: name, New: pair[0] == nil, Gone: pair[1] == nil}
		for i, f := range pair {
			if f == nil {
				continue
			}
			text, err := f.Contents()
			if err != nil {
				return Commit{}, nil, err
			}
			if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
				v.Binary = true
				continue
			}
			if i == 0 {
				v.Was = text
			} else {
				v.Now = text
			}
		}
		if v.Binary {
			v.Was, v.Now = "", ""
		}
		versions = append(versions, v)
	}
	slices.Sort(names)
	slices.SortFunc(versions, func(a, b Version) int { return strings.Compare(a.Path, b.Path) })
	return read(c, names), versions, nil
}

// hasDot says whether any part of a path starts with a dot.
func hasDot(p string) bool {
	return slices.ContainsFunc(strings.Split(p, "/"), func(part string) bool { return strings.HasPrefix(part, ".") })
}

// History gives a person every commit that touched the office's files,
// newest first: each change, with who made it, why and who decided, and
// each note's write. Only the people who decide a change read it.
func (c *Changes) History(viewer string) ([]Commit, error) {
	if err := c.mayRead(viewer); err != nil {
		return nil, err
	}
	r, err := openRepo(c.dir)
	if err != nil {
		return nil, err
	}
	return r.log()
}

// Commit gives a person one commit of the history, with each of the
// office's files it touched as it was and as it is. Only the people who
// decide a change read it.
func (c *Changes) Commit(viewer, id string) (Commit, []Version, error) {
	if err := c.mayRead(viewer); err != nil {
		return Commit{}, nil, err
	}
	r, err := openRepo(c.dir)
	if err != nil {
		return Commit{}, nil, err
	}
	return r.show(id)
}

// mayRead says why a person may not read the history, or nothing when they
// may: it holds every file of the office as it was, every team's notes
// among them, and is read only by the people who decide a change.
func (c *Changes) mayRead(viewer string) error {
	o, err := c.office()
	if err != nil {
		return err
	}
	if !slices.Contains(Deciders(o), viewer) {
		return refuse("%s does not decide changes to the office, so cannot read its history", viewer)
	}
	return nil
}

// ByHand gives the office's files that are not as the last commit left
// them, read from the folder now: those edited, added or removed by hand
// and not yet committed.
func (c *Changes) ByHand() ([]string, error) {
	r, err := openRepo(c.dir)
	if err != nil {
		return nil, err
	}
	return r.byHand(c.dir)
}
