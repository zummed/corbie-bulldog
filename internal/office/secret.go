package office

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SecretFile is where an office's secret is kept: in Bulldog's own
// directory in the user's config directory, under the office's id.
func SecretFile(id string) (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "corbie-bulldog", id, "secret"), nil
}

// Secret gives the office's secret, which signs keys and keys the hashes
// Bulldog writes in labels, run names and refs. It is made the first time,
// and kept in SecretFile, never in the office's directory, which is
// edited, copied and put in git.
func Secret(id string) ([]byte, error) {
	file, err := SecretFile(id)
	if err != nil {
		return nil, err
	}
	written, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		written, err = makeSecret(file)
	}
	if err != nil {
		return nil, err
	}
	secret, err := hex.DecodeString(strings.TrimSpace(string(written)))
	if err != nil || len(secret) != 32 {
		return nil, fmt.Errorf("%s does not hold an office's secret: remove it, and a new one is made, which signs everyone out", file)
	}
	return secret, nil
}

// makeSecret writes a new secret at a file and gives what the file holds.
// The file is written whole beside it and linked into place, so a second
// Bulldog making it at the same moment reads the first one's.
func makeSecret(file string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".secret-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	secret := make([]byte, 32)
	rand.Read(secret)
	_, err = tmp.WriteString(hex.EncodeToString(secret) + "\n")
	if err == nil {
		err = tmp.Chmod(0o600)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Link(tmp.Name(), file)
	}
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	return os.ReadFile(file)
}
