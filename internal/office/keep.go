package office

import "sync"

// A Keeper keeps an office's latest good reading, and the problems of its
// latest reading, and reads the folder again when its files change. A
// reading replaces the last one whole: whoever holds a reading holds all
// of one, and none of another.
type Keeper struct {
	dir string

	// looking is held for the whole of a look, so that one look's reading
	// is never kept over a later one's.
	looking sync.Mutex

	mu     sync.Mutex
	mark   string
	office *Office
	err    error
}

// Keep reads the office in a folder, and keeps it.
func Keep(dir string) *Keeper {
	k := &Keeper{dir: dir}
	k.Look()
	return k
}

// Look reads the office again if any of its files has been written,
// added, removed or renamed since the last reading, and says whether a
// good reading replaced the last. A reading that is refused leaves the
// office as it was last read well.
func (k *Keeper) Look() bool {
	k.looking.Lock()
	defer k.looking.Unlock()
	mark, err := Mark(k.dir)
	k.mu.Lock()
	same := err == nil && mark == k.mark && k.mark != ""
	k.mu.Unlock()
	if same {
		return false
	}
	// The mark is taken before the files are read, so a file written while
	// they are read is read again at the next look.
	o, err := Read(k.dir)
	k.mu.Lock()
	defer k.mu.Unlock()
	k.mark, k.err = mark, err
	if o == nil {
		return false
	}
	k.office = o
	return true
}

// Office gives the latest good reading, or nil while the office has never
// read well.
func (k *Keeper) Office() *Office {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.office
}

// Problems gives the problems of the latest reading, whether it was good
// or refused. A folder that cannot be read is one problem, saying why.
func (k *Keeper) Problems() Problems {
	k.mu.Lock()
	defer k.mu.Unlock()
	switch problems, ok := k.err.(Problems); {
	case ok:
		return problems
	case k.err != nil:
		return Problems{{Text: k.err.Error(), Refuses: true}}
	case k.office != nil:
		return k.office.Problems
	}
	return nil
}
