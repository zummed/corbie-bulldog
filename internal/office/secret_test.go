package office

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestOfficeSecretNeverInOffice: the secret is made at the first start in
// Bulldog's own directory in the user's config directory, under the
// office's id, and nothing of it is written in the office's directory. It
// sets the user's config directory, so it runs alone.
func TestOfficeSecretNeverInOffice(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)
	t.Setenv("AppData", config)
	o := read(t, house())
	before := tree(t, o.Dir)

	secret, err := Secret(o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 32 {
		t.Errorf("the secret is %d bytes", len(secret))
	}
	file, err := secretFile(o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(config, file); err != nil || filepath.Base(filepath.Dir(file)) != o.ID || !filepath.IsLocal(rel) {
		t.Errorf("the secret is kept at %s, not under %s and the office's id", file, config)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the secret's file is %v (%v)", info.Mode(), err)
	}
	again, err := Secret(o.ID)
	if err != nil || !bytes.Equal(again, secret) {
		t.Errorf("the secret given again differs (%v)", err)
	}
	other, err := Secret("0123456789abcdef")
	if err != nil || bytes.Equal(other, secret) {
		t.Errorf("another office has the same secret (%v)", err)
	}

	after := tree(t, o.Dir)
	if len(after) != len(before) {
		t.Errorf("making the secret wrote in the office's directory: %v, then %v", before, after)
	}
	written, _ := os.ReadFile(file)
	for name, held := range after {
		if bytes.Contains(held, bytes.TrimSpace(written)) {
			t.Errorf("%s holds the secret", name)
		}
	}

	os.WriteFile(file, []byte("not a secret\n"), 0o600)
	if _, err := Secret(o.ID); err == nil {
		t.Error("a file that holds no secret is taken")
	}
}

// tree gives every file under a directory, with what it holds.
func tree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(file string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		files[file], err = os.ReadFile(file)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
