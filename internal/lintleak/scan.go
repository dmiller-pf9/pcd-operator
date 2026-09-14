package lintleak

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Finding is one banned word in one place.
type Finding struct {
	Path string
	Line int
	Term string
	Why  string
	Text string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: banned term %q (%s): %s", f.Path, f.Line, f.Term, f.Why, f.Text)
}

// ScanText reports every banned term in text. Path is used only for reporting.
func ScanText(path, text string) []Finding {
	var found []Finding
	for i, line := range strings.Split(text, "\n") {
		for _, term := range terms {
			if term.Pattern.MatchString(line) {
				found = append(found, Finding{
					Path: path,
					Line: i + 1,
					Term: term.Name,
					Why:  term.Why,
					Text: strings.TrimSpace(line),
				})
			}
		}
	}
	return found
}

// ScanTree reports every banned term in every regular file under root.
// Generated output directories are deliberately not scanned by callers — see
// the comment on ScanAll.
//
// A missing root is not an error: the api/ tree does not exist until milestone
// 002, and milestone 001 must still pass its own verification.
func ScanTree(root string) ([]Finding, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}
	var found []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if skipFile(d.Name()) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found = append(found, ScanText(path, string(b))...)
		return nil
	})
	return found, err
}

// skipDir excludes trees whose contents are not ours to police.
func skipDir(name string) bool {
	switch name {
	case ".git", ".claude", "bin", "testbin", "vendor", "testdata":
		return true
	}
	return false
}

// skipFile excludes test sources. Test fixtures legitimately contain banned
// words — that is how the linter proves it catches them — and no string in a
// test reaches a user.
func skipFile(name string) bool {
	return strings.HasSuffix(name, "_test.go")
}
