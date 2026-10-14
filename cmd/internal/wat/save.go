package wat

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type fileState struct {
	data []byte
	mode os.FileMode
}

type Draft struct {
	books    *Books
	files    map[string]fileState
	accounts []string
	id       string
	text     string
	saved    bool
}

func (b *Books) snapshot() (map[string]fileState, error) {
	text, err := b.run(b.Config.Journal, "files")
	if err != nil {
		return nil, err
	}
	files := make(map[string]fileState)
	for _, path := range strings.Split(strings.TrimSpace(text), "\n") {
		if !filepath.IsAbs(path) {
			path = filepath.Join(b.Config.root, path)
		}
		path = filepath.Clean(path)
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("resolve journal: %w", err)
		}
		if !within(b.Config.root, path) || path != real {
			return nil, fmt.Errorf("journals must be nonsymlink files inside the configuration directory: %s", path)
		}
		if ext := filepath.Ext(path); ext != ".journal" && ext != ".ledger" {
			return nil, fmt.Errorf("wat supports journal inputs only (.journal or .ledger): %s", path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("not a regular journal file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files[path] = fileState{data: data, mode: info.Mode().Perm()}
	}
	if _, ok := files[b.Config.WriteJournal]; !ok {
		return nil, fmt.Errorf("write_journal must exist and be included in journal")
	}
	return files, nil
}

func (b *Books) NewDraft() (*Draft, error) {
	files, err := b.snapshot()
	if err != nil {
		return nil, err
	}
	if err := b.Check(); err != nil {
		return nil, err
	}
	accounts, err := b.Accounts()
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	return &Draft{books: b, files: files, accounts: accounts, id: id}, nil
}

func (d *Draft) Preview(entry Entry) (string, error) {
	d.text = ""
	if d.saved {
		return "", fmt.Errorf("this transaction has already been saved")
	}
	text, err := entry.render(d.books.Config.Year, d.accounts, d.id)
	if err != nil {
		return "", err
	}
	d.text = text
	return text, nil
}

func (d *Draft) unchanged() error {
	current, err := d.books.snapshot()
	if err != nil {
		return err
	}
	if len(current) != len(d.files) {
		return fmt.Errorf("included journals changed; cancel and reopen the transaction")
	}
	for path, original := range d.files {
		state, ok := current[path]
		if !ok || state.mode != original.mode || !bytes.Equal(state.data, original.data) {
			return fmt.Errorf("journal changed: %s; cancel and reopen the transaction", path)
		}
	}
	return nil
}

func (d *Draft) proposed() []byte {
	return d.proposedText(d.text)
}

func (d *Draft) proposedText(text string) []byte {
	original := d.files[d.books.Config.WriteJournal].data
	separator := "\n"
	if len(original) != 0 && original[len(original)-1] != '\n' {
		separator = "\n\n"
	}
	return append(append(append([]byte{}, original...), separator...), text...)
}

func (d *Draft) validate(data []byte) (err error) {
	root, err := os.MkdirTemp("", "wat-validate-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(root)) }()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	root = resolvedRoot
	staged := make(map[string]bool)
	var journal string
	for path, state := range d.files {
		relative, err := filepath.Rel(d.books.Config.root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, relative)
		staged[target] = true
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		content := state.data
		if path == d.books.Config.WriteJournal {
			content = data
		}
		if err := os.WriteFile(target, content, 0600); err != nil {
			return err
		}
		if path == d.books.Config.Journal {
			journal = target
		}
	}
	// Let hledger resolve includes; do not implement a second journal parser.
	// Absolute or escaping includes must not validate against the live books.
	listing, err := d.books.run(journal, "files")
	if err != nil {
		return err
	}
	included := make(map[string]bool)
	for _, path := range strings.Split(strings.TrimSpace(listing), "\n") {
		path = filepath.Clean(path)
		if !staged[path] {
			return fmt.Errorf("include escapes the validation workspace: %s; use relative journal includes", path)
		}
		included[path] = true
	}
	if len(included) != len(staged) {
		return fmt.Errorf("journal include set changed in validation")
	}
	_, err = d.books.run(journal, "check", "--strict")
	return err
}

func writeSynced(path string, data []byte, mode os.FileMode) (err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func (d *Draft) Save() (backup string, err error) {
	if d.saved || d.text == "" {
		return "", fmt.Errorf("preview a new transaction before saving")
	}
	target := d.books.Config.WriteJournal
	lockPath := target + ".wat-lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("acquire journal lock (another wat writer or a stale lock): %w", err)
	}
	if err := lock.Close(); err != nil {
		return "", errors.Join(err, os.Remove(lockPath))
	}
	defer func() {
		if removeErr := os.Remove(lockPath); removeErr != nil {
			if d.saved {
				removeErr = fmt.Errorf("journal saved, but lock cleanup failed: %w", removeErr)
			}
			err = errors.Join(err, removeErr)
		}
	}()
	if err := d.unchanged(); err != nil {
		return "", err
	}
	data := d.proposed()
	if err := d.validate(data); err != nil {
		return "", fmt.Errorf("proposed books failed validation: %w", err)
	}
	directory := filepath.Dir(target)
	temporary, err := os.CreateTemp(directory, ".wat-save-")
	if err != nil {
		return "", err
	}
	tempPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return "", errors.Join(err, os.Remove(tempPath))
	}
	defer func() {
		if !d.saved {
			err = errors.Join(err, os.Remove(tempPath))
		}
	}()
	state := d.files[target]
	if err := writeSynced(tempPath, data, state.mode); err != nil {
		return "", err
	}
	previous, err := os.CreateTemp(directory, ".wat-backup-")
	if err != nil {
		return "", err
	}
	backup = previous.Name()
	if err := previous.Close(); err != nil {
		return backup, err
	}
	if err := writeSynced(backup, state.data, 0600); err != nil {
		return backup, fmt.Errorf("write recovery copy: %w", err)
	}
	if err := d.unchanged(); err != nil {
		return backup, err
	}
	if err := os.Rename(tempPath, target); err != nil {
		return backup, fmt.Errorf("replace journal: %w", err)
	}
	d.saved = true
	dir, err := os.Open(directory)
	if err != nil {
		return backup, fmt.Errorf("journal saved, but directory sync failed: %w", err)
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return backup, fmt.Errorf("journal saved, but directory sync failed: %w", err)
	}
	return backup, nil
}
