package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	bolt "go.etcd.io/bbolt"
)

const historyFile = ".go-test.db"

// HistoryEntry is a single entry in the history file.
type HistoryEntry struct {
	Timestamp time.Time
	Path      string
	Args      []string
	Dir       string
}

// String returns a string representation of the HistoryEntry.
func (h HistoryEntry) String() string {
	return fmt.Sprintf("%s - %s", h.Timestamp.Format("01/02/2006 @ 15:04:05"), strings.Join(h.Args, " "))
}

// JSON converts the HistoryEntry to a JSON byte slice.
func (h HistoryEntry) JSON() []byte {
	data, err := json.Marshal(h)
	if err != nil {
		panic(err)
	}

	return data
}

// Load loads the HistoryEntry from a JSON byte slice.
func (h *HistoryEntry) Load(data []byte) {
	err := json.Unmarshal(data, h)
	if err != nil {
		panic(err)
	}
}

// Hash returns a hash of the HistoryEntry.
func (h HistoryEntry) Hash() string {
	hash := md5.Sum([]byte(fmt.Sprintf("%s-%s-%s", h.Path, strings.Join(h.Args, " "), h.Dir)))
	key := fmt.Sprintf("%x", hash)

	return key
}

func getHistoryFile(file string) *bolt.DB {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	p := filepath.Join(home, file)

	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		panic(err)
	}

	return db
}

func logRunHistory(command exec.Cmd) {
	he := HistoryEntry{
		Path:      command.Path,
		Args:      command.Args,
		Dir:       command.Dir,
		Timestamp: time.Now(),
	}

	file := getHistoryFile(historyFile)
	defer file.Close()

	// write the command to the file
	err := file.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("history"))
		if err != nil {
			return err
		}

		// todo: limit this to n entries using a config
		// key is a hash of the path, args, and dir.
		key := he.Hash()

		b.Put([]byte(key), he.JSON())

		return nil
	})
	if err != nil {
		panic(err)
	}
}

func selectHistory() (HistoryEntry, error) {
	file := getHistoryFile(historyFile)
	defer file.Close()

	var entries []HistoryEntry
	err := file.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("history"))
		if b == nil {
			return nil
		}

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var he HistoryEntry
			he.Load(v)

			entries = append(entries, he)
		}

		return nil
	})
	if err != nil {
		return HistoryEntry{}, fmt.Errorf("error retrieving history %w", err)
	}

	subtestPrompt := promptui.Select{
		Label: "Run from history",
		Items: entries,
		Templates: &promptui.SelectTemplates{
			// Label:    `{{ .Timestamp.Format "01/02/2006 @ 3:4:5" }}`,
			Active:   "\U0001F449 {{ . }}",
			Inactive: `{{ . }}`,
			Selected: "{{ . }}",
		},
		Searcher: func(input string, index int) bool {
			test := entries[index]
			if strings.Contains(strings.ToLower(test.String()), strings.ToLower(input)) {
				return true
			}

			return false
		},
	}

	index, _, err := subtestPrompt.Run()
	switch {
	case err == nil:
	case err == promptui.ErrInterrupt:
		fmt.Println("No history selected. Exiting.")
		os.Exit(0)
	default:
		return HistoryEntry{}, fmt.Errorf("error selecting history %w", err)
	}

	return entries[index], nil
}

func getLastCommand() (HistoryEntry, error) {
	file := getHistoryFile(historyFile)
	defer file.Close()

	var lastCommand HistoryEntry
	err := file.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("history"))
		if b == nil {
			return nil
		}

		b.ForEach(func(k, v []byte) error {
			var he HistoryEntry
			he.Load(v)

			if lastCommand.Timestamp.Before(he.Timestamp) {
				lastCommand = he
			}

			return nil
		})

		return nil
	})
	if err != nil {
		return HistoryEntry{}, fmt.Errorf("error viewing history file %w", err)
	}

	return lastCommand, nil
}

func runHistoryEntry(he HistoryEntry) {
	cmd := exec.Cmd{
		Path:   he.Path,
		Args:   he.Args,
		Dir:    he.Dir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	fmt.Println("Running", cmd.Args, "@", cmd.Dir)

	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	// do nothing
	case errors.As(err, &exit):
	// do nothing
	default:
		panic(err)
	}
}
