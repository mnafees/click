package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxEntries = 500

type History struct {
	entries []string
	pos     int
	path    string
}

func New() *History {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".click_history")
	h := &History{path: path, pos: -1}
	h.load()
	return h
}

func (h *History) Add(query string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	// dedupe: remove if already present at end
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == q {
		h.pos = -1
		return
	}
	h.entries = append(h.entries, q)
	if len(h.entries) > maxEntries {
		h.entries = h.entries[len(h.entries)-maxEntries:]
	}
	h.pos = -1
	h.save()
}

// Prev moves backward in history and returns the entry. Returns ("", false) at the start.
func (h *History) Prev() (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.pos == -1 {
		h.pos = len(h.entries) - 1
	} else if h.pos > 0 {
		h.pos--
	} else {
		return h.entries[0], true
	}
	return h.entries[h.pos], true
}

// Next moves forward in history. Returns ("", false) when past the end (back to editing).
func (h *History) Next() (string, bool) {
	if h.pos == -1 {
		return "", false
	}
	h.pos++
	if h.pos >= len(h.entries) {
		h.pos = -1
		return "", false
	}
	return h.entries[h.pos], true
}

func (h *History) Pos() int {
	return h.pos
}

func (h *History) Reset() {
	h.pos = -1
}

func (h *History) load() {
	f, err := os.Open(h.path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			h.entries = append(h.entries, line)
		}
	}
}

func (h *History) save() {
	f, err := os.Create(h.path)
	if err != nil {
		return
	}
	defer f.Close()
	for _, e := range h.entries {
		// replace newlines with spaces for single-line storage
		fmt.Fprintln(f, strings.ReplaceAll(e, "\n", " "))
	}
}
