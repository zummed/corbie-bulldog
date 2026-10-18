package server

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// entryFile serves a file on an entry to whoever may see the entry, so
// that it can do nothing: its type is told from what it holds, never its
// name; only a picture other than SVG, or a PDF, is shown, and anything
// else is given to be saved; and nothing in it may run.
func (s *site) entryFile(person string, w http.ResponseWriter, r *http.Request) {
	entry, index, ok := strings.Cut(r.PathValue("id"), "-")
	id, err := strconv.ParseInt(entry, 10, 64)
	i, err2 := strconv.Atoi(index)
	if !ok || err != nil || err2 != nil {
		s.refuse(w, r, errNotThere)
		return
	}
	e, err := s.Requests.Entry(id)
	if err != nil {
		s.refuse(w, r, err)
		return
	}
	if i < 0 || i >= len(e.Files) {
		s.refuse(w, r, errNotThere)
		return
	}
	name := e.Files[i].Name
	data, err := s.Requests.File(e.Request, id, name, person)
	if err != nil {
		s.refuse(w, r, err)
		return
	}
	kind := http.DetectContentType(data)
	shown := strings.HasPrefix(kind, "image/") || kind == "application/pdf"
	disposition := "attachment"
	if shown {
		disposition = "inline"
	} else {
		kind = "application/octet-stream"
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(data)
}

// The screens: plain pages, styles and scripts, in the program.
//
//go:embed screens
var screenFiles embed.FS

// static serves one of the screens' own files, by its name: a folder is
// not listed.
func static(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	data, err := fs.ReadFile(screenFiles, "screens/"+name)
	if err != nil || name == "index.html" {
		answer(w, http.StatusNotFound, notThere)
		return
	}
	w.Header().Set("Content-Type", screenType(name))
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

// screenType gives the type of one of the screens' files: a font is
// named here, since not every computer's table of types knows it.
func screenType(name string) string {
	if path.Ext(name) == ".woff2" {
		return "font/woff2"
	}
	return mime.TypeByExtension(path.Ext(name))
}

// theScreens serves the screens, to a person signed in.
func theScreens(w http.ResponseWriter) {
	data, err := fs.ReadFile(screenFiles, "screens/index.html")
	if err != nil {
		answer(w, http.StatusInternalServerError, "The screens are missing from the program: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}
