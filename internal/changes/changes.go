// Package changes is the one way the office's files change. A change is
// drafted, by a worker with the office tools or by a person on the
// screens, as the whole new text of each file it touches; checked, by
// reading the office as it would make it; put to be decided, and approved
// only by a person; written, every file or none; and kept, as a commit in
// the office's repository that says who, why and who decided. A team's
// note is written the same way with the decision left out. The history is
// read and written with a library, and no git program is run.
package changes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

// Changes is an office's changes: the drafts of its requests, the changes
// put to be decided, and its history.
type Changes struct {
	dir    string
	db     *sql.DB
	keeper *office.Keeper

	// Now is the clock commits and puts are dated by.
	Now func() time.Time
	// Decide hands a change that is put on to be decided. Its decision
	// comes back to Decided.
	Decide func(Put) error
	// Told is told how each change put ended, once it has: written,
	// refused, not written or withdrawn. Whoever decides drops a
	// withdrawn one.
	Told func(Outcome)

	// writing is held while anything is written into the folder, so that
	// two writes never check and write over each other; deciding while a
	// change is kept as put or decided, so that each ends once; putting
	// while a change is put and handed on to be decided.
	writing  sync.Mutex
	deciding sync.Mutex
	putting  sync.Mutex
}

// Open readies an office's changes in the store, refusing a store whose
// changes another Bulldog kept. The keeper's office is the one a worker's
// call is answered in, and the keeper is told to read the office again
// once a change is written, before anyone is told.
func Open(ctx context.Context, db *sql.DB, dir string, keeper *office.Keeper) (*Changes, error) {
	if err := ready(ctx, db); err != nil {
		return nil, err
	}
	return &Changes{dir: dir, db: db, keeper: keeper, Now: time.Now}, nil
}

// A Call is one call of an office tool: the tool, what the model gave
// it, the worker that called it and the request it works on.
type Call struct {
	Request int64
	Worker  string
	Tool    string
	Args    json.RawMessage
}

// An Answer is a tool's result.
type Answer struct {
	Text string
}

// SearchLimit is how many characters search_files gives.
const SearchLimit = 20_000

// The office tools, as the tools files Bulldog ships write them, those
// that read and those that write.
const (
	ToolFiles  = "list_files"
	ToolRead   = "read_file"
	ToolSearch = "search_files"
	ToolManual = "manual"
	ToolWrite  = "write_file"
	ToolRemove = "remove_file"
	ToolCheck  = "check_draft"
	ToolPut    = "put_draft"
	// ToolWriteDoc is the docs tools' write, which this area writes as a
	// change of one file.
	ToolWriteDoc = "write_doc"
)

// Tools are the office tools this area answers, in the order they sort.
func Tools() []string {
	tools := []string{ToolFiles, ToolRead, ToolSearch, ToolManual, ToolWrite, ToolRemove, ToolCheck, ToolPut}
	slices.Sort(tools)
	return tools
}

// Answer does one call of an office tool and gives its result. What the
// worker did wrong is said in the result, for it to mend; an error is
// what went wrong in Bulldog, for a person to read.
func (c *Changes) Answer(ctx context.Context, call Call) (Answer, error) {
	var args struct {
		Path  string `json:"path"`
		Text  string `json:"text"`
		Why   string `json:"why"`
		Page  string `json:"page"`
		Words string `json:"words"`
	}
	if len(call.Args) > 0 && string(call.Args) != "null" {
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return Answer{Text: fmt.Sprintf("%s could not read what it was given: %v", call.Tool, err)}, nil
		}
	}
	said := func(text string, err error) (Answer, error) { return Answer{Text: text}, err }
	// The office as last read well is the one the worker runs in.
	o := c.keeper.Office()
	if o == nil {
		return Answer{}, errors.New("the office has never read well, so no office tool is answered")
	}
	team := ""
	if w := o.Workers[call.Worker]; w != nil {
		team = w.Team
	}
	reach := func(file string) string {
		if !o.Reaches(team, file) {
			return fmt.Sprintf("%s is outside what you reach, the files of your own team's folder and of the teams within it, so it is not read or drafted", file)
		}
		return ""
	}
	switch call.Tool {
	case ToolFiles:
		return said(c.files(ctx, call.Request, o, team))
	case ToolRead:
		if problem := reach(args.Path); problem != "" {
			return said(problem, nil)
		}
		return said(c.read(ctx, call.Request, args.Path))
	case ToolSearch:
		return said(c.search(ctx, call.Request, o, team, args.Words))
	case ToolWrite, ToolRemove:
		if problem := reach(args.Path); problem != "" {
			return said(problem, nil)
		}
		if call.Tool == ToolRemove {
			return said(c.write(ctx, call.Request, args.Path, nil))
		}
		return said(c.write(ctx, call.Request, args.Path, &args.Text))
	case ToolCheck:
		files, err := drafted(ctx, c.db, call.Request)
		if err != nil {
			return Answer{}, err
		}
		check, err := c.Check(ctx, files, Drafter{Name: call.Worker})
		if err != nil {
			return Answer{}, err
		}
		if err := keepTaken(ctx, c.db, call.Request, files, check.Files); err != nil {
			return Answer{}, err
		}
		return Answer{Text: check.Text()}, nil
	case ToolPut, ToolWriteDoc:
		return c.written(ctx, call)
	case ToolManual:
		return Answer{Text: Manual(args.Page)}, nil
	}
	return Answer{Text: fmt.Sprintf("there is no office tool %s: they are %s", call.Tool, strings.Join(Tools(), ", "))}, nil
}

// Drafted gives a request's draft, a file to each path, in the order of
// their paths.
func (c *Changes) Drafted(ctx context.Context, request int64) ([]File, error) {
	return drafted(ctx, c.db, request)
}

// folder reads every file of the office in the folder now, and the
// problems of links it left out.
func (c *Changes) folder() (map[string][]byte, office.Problems, error) {
	return office.Folder(c.dir)
}

// files lists every file of the office a team's worker reaches, and what
// the draft does to each.
func (c *Changes) files(ctx context.Context, request int64, o *office.Office, team string) (string, error) {
	draft, err := drafted(ctx, c.db, request)
	if err != nil {
		return "", err
	}
	folder, _, err := c.folder()
	if err != nil {
		return "", err
	}
	does := map[string]string{}
	for name := range folder {
		if o.Reaches(team, name) {
			does[name] = IsAsItIs
		}
	}
	for _, f := range draft {
		does[f.Path] = f.Does()
	}
	var b strings.Builder
	b.WriteString("Every file of the office you reach, and what your draft does to it:\n")
	for _, name := range sortedKeys(does) {
		fmt.Fprintf(&b, "%s: %s\n", name, does[name])
	}
	return b.String(), nil
}

// read gives one file as the draft has it, or as it is.
func (c *Changes) read(ctx context.Context, request int64, file string) (string, error) {
	if problem := refusedPath(file); problem != "" {
		return problem, nil
	}
	draft, err := drafted(ctx, c.db, request)
	if err != nil {
		return "", err
	}
	for _, f := range draft {
		if f.Path == file && f.Gone {
			return fmt.Sprintf("your draft removes %s", file), nil
		}
		if f.Path == file {
			return f.Text, nil
		}
	}
	folder, _, err := c.folder()
	if err != nil {
		return "", err
	}
	contents, ok := folder[file]
	switch {
	case !ok:
		return fmt.Sprintf("there is no file %s in the office or in your draft: list_files lists them", file), nil
	case !utf8.Valid(contents):
		return fmt.Sprintf("%s is not text, and cannot be read here", file), nil
	}
	return string(contents), nil
}

// search gives the lines of the files of the office a team's worker
// reaches that hold the words asked for, in any case, each with its file
// and its line, as the draft has them, or as they are, as many as fit in
// SearchLimit characters.
func (c *Changes) search(ctx context.Context, request int64, o *office.Office, team, words string) (string, error) {
	words = strings.TrimSpace(words)
	if words == "" {
		return "A search needs words, and this one had none, so nothing was searched.", nil
	}
	draft, err := drafted(ctx, c.db, request)
	if err != nil {
		return "", err
	}
	folder, _, err := c.folder()
	if err != nil {
		return "", err
	}
	files := overlay(folder, draft)
	var lines []string
	for _, name := range sortedKeys(files) {
		if !o.Reaches(team, name) || refusedPath(name) != "" || !utf8.Valid(files[name]) {
			continue
		}
		for i, line := range strings.Split(string(files[name]), "\n") {
			if strings.Contains(strings.ToLower(line), strings.ToLower(words)) {
				lines = append(lines, fmt.Sprintf("%s:%d: %s", name, i+1, line))
			}
		}
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No line of the files you reach holds %q.", words), nil
	}
	var b strings.Builder
	shown := 0
	for room := SearchLimit; shown < len(lines); shown++ {
		line := []rune(lines[shown] + "\n")
		if len(line) > room && shown > 0 {
			break
		}
		if len(line) > room {
			line = append(line[:room-1], '\n')
		}
		b.WriteString(string(line))
		room -= len(line)
	}
	if left := len(lines) - shown; left > 0 {
		fmt.Fprintf(&b, "%d more lines hold it; search for something narrower, or read a file with read_file.", left)
	}
	return b.String(), nil
}

// write puts the whole new text of a file into a request's draft, or that
// it goes when text is nil, drafted against the file as it is in the folder
// now: a file that has changed there since it was first drafted is drafted
// anew, and the builder is told. A file written as it is in the folder, or
// the removal of a file only the draft made, leaves the draft.
func (c *Changes) write(ctx context.Context, request int64, file string, text *string) (string, error) {
	if problem := refusedPath(file); problem != "" {
		return problem, nil
	}
	folder, _, err := c.folder()
	if err != nil {
		return "", err
	}
	if problem := placeProblem(folder, file); problem != "" {
		return problem, nil
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	draft, err := drafted(ctx, tx, request)
	if err != nil {
		return "", err
	}
	now, has := folder[file]
	f := File{Path: file, Was: string(now), Had: has}
	anew := ""
	for _, d := range draft {
		if d.Path == file && (d.Had != has || d.Was != string(now)) {
			anew = fmt.Sprintf("%s has changed in the folder since you first drafted it, so it is drafted anew, against the folder as it is now: whoever decides is shown your text beside the folder's. ", file)
		}
	}
	var said string
	switch {
	case text == nil && !has && !slices.ContainsFunc(draft, func(d File) bool { return d.Path == file }):
		return fmt.Sprintf("there is no file %s in the office or in your draft, so there is nothing to remove", file), nil
	case text == nil && !has:
		err, said = dropDraft(ctx, tx, request, file), fmt.Sprintf("%s was only in your draft, and is taken out of it", file)
	case text == nil:
		f.Gone = true
		err, said = keepDraft(ctx, tx, request, f), anew+fmt.Sprintf("your draft removes %s. Nothing changes in the office until a person says yes to the draft", file)
	case has && *text == string(now):
		err, said = dropDraft(ctx, tx, request, file), fmt.Sprintf("that is %s as it is, so your draft leaves it as it is", file)
	default:
		f.Text = *text
		said = anew + fmt.Sprintf("%s is in your draft, %s. Nothing changes in the office until a person says yes to the draft: check_draft reads the office as it would make it", file, f.Does())
		err = keepDraft(ctx, tx, request, f)
	}
	if err != nil {
		return "", err
	}
	return said, tx.Commit()
}

// refusedPath says why a draft may not hold a path, or nothing when it may:
// a draft holds only files of the office, written plainly by their paths
// in its folder.
func refusedPath(file string) string {
	clean := path.Clean(file)
	first, _, _ := strings.Cut(clean, "/")
	switch {
	case file == "":
		return "a file is named by its path in the office, such as garden/gardener.yaml, and no path was given"
	case path.IsAbs(file) || clean == ".." || strings.HasPrefix(clean, "../"):
		return fmt.Sprintf("%s is outside the office's folder, and a draft holds only files of the office", file)
	case clean != file || strings.Contains(file, "\\"):
		return fmt.Sprintf("%q is not a path written plainly: write it as %s", file, strings.ReplaceAll(clean, "\\", "/"))
	case first == shipped.Dir:
		return fmt.Sprintf("%s is Bulldog's own: the files it ships and its store, which no change touches", file)
	case first == ".git":
		return fmt.Sprintf("%s is the office's repository, which no change touches", file)
	case hasDot(file):
		return fmt.Sprintf("%s is not a file of the office: nothing whose name starts with a dot is, so it would be written and never read", file)
	}
	return ""
}

// placeProblem says why a path cannot be a file in the folder: a folder is
// there, or a file is where one of its folders would be.
func placeProblem(folder map[string][]byte, file string) string {
	for name := range folder {
		if strings.HasPrefix(name, file+"/") {
			return fmt.Sprintf("%s is a folder of the office, and a draft holds files", file)
		}
	}
	for d := path.Dir(file); d != "."; d = path.Dir(d) {
		if _, ok := folder[d]; ok {
			return fmt.Sprintf("%s is a file, so nothing can be in it as %s", d, file)
		}
	}
	return ""
}

// overlay gives the office's files with a change laid over them.
func overlay(folder map[string][]byte, files []File) map[string][]byte {
	out := make(map[string][]byte, len(folder))
	for name, contents := range folder {
		out[name] = contents
	}
	for _, f := range files {
		if f.Gone {
			delete(out, f.Path)
		} else {
			out[f.Path] = []byte(f.Text)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// A Refusal is a change, or something asked of one, that this area does
// not allow, said in a sentence for whoever asked. Nothing was written.
type Refusal string

func (r Refusal) Error() string { return string(r) }

func refuse(format string, args ...any) error { return Refusal(fmt.Sprintf(format, args...)) }

// ErrNotThere is what anyone is told of a change the office's history does
// not hold.
var ErrNotThere = errors.New("there is no such change in the office's history")

// errNoDecide says nothing was given to decide changes.
var errNoDecide = errors.New("nothing decides the office's changes: the server gives Changes no Decide")
