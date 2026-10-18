package office

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// root gives the folder an office is in, with every link in its path
// followed: the folder its files are read from, and looked at for changes.
func root(dir string) (string, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s is not there", dir)
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", dir)
	}
	return filepath.EvalSymlinks(dir)
}

// Folder reads every file of the office in a folder, by its path in it.
// Nothing whose name starts with a dot is the office's. A link is read
// where it points when that is a file of the office; one that points to
// nothing, outside the folder or into what is not the office's is left
// out, with a problem saying so.
func Folder(dir string) (map[string][]byte, Problems, error) {
	root, err := root(dir)
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(filepath.Join(root, OfficeFile)); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("%s holds no %s, so it is not an office", dir, OfficeFile)
	}
	files := map[string][]byte{}
	var problems Problems
	err = filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file == root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, file)
		name := filepath.ToSlash(rel)
		if d.Type()&fs.ModeSymlink != 0 {
			left := func(why string) error {
				problems = append(problems, Problem{File: name, Team: Top, Text: "it is a link " + why + ", so it is not a file of the office"})
				return nil
			}
			target, err := filepath.EvalSymlinks(file)
			if err != nil {
				return left("to nothing that is there")
			}
			inside, _ := filepath.Rel(root, target)
			switch inside = filepath.ToSlash(inside); {
			case inside == ".." || strings.HasPrefix(inside, "../"):
				return left("to somewhere outside the office's folder")
			case hasDot(inside):
				return left("to " + inside + ", and nothing whose name starts with a dot is the office's")
			}
			if info, err := os.Stat(target); err != nil || !info.Mode().IsRegular() {
				return nil
			}
		} else if !d.Type().IsRegular() {
			return nil
		}
		contents, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files[name] = contents
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return files, problems, nil
}

// Mark gives what the files of the office in a folder are now: it differs
// whenever any of them is written, added, removed or renamed, whatever
// their times. Reading the same files twice gives the same mark. It looks
// at the folder Folder reads, and a link inside it is looked at where it
// points by looking at what it points to, which is in the folder too.
func Mark(dir string) (string, error) {
	root, err := root(dir)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%q\n", root)
	err = filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, file)
		fmt.Fprintf(h, "%q %d %d %v\n", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano(), info.Mode())
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
