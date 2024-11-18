package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	bolt "go.etcd.io/bbolt"
)

const historyFile = ".go-test.db"

// HistoryEntry is a single entry in the history file.
type HistoryEntry struct {
	Timestamp     time.Time
	Path          string
	Args          []string
	Dir           string
	LastRunStatus bool
}

// String returns a string representation of the HistoryEntry.
func (h HistoryEntry) String() string {
	return fmt.Sprintf("%s - %s %s", h.Timestamp.Format("01/02/2006 @ 15:04:05"), strings.Join(h.Args, " "),
		statusToStr(h.LastRunStatus))
}

func statusToStr(b bool) string {
	if b {
		return "✅"
	}

	return "❌"
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

func logRunHistory(command exec.Cmd, pass bool) {
	he := HistoryEntry{
		Path:          command.Path,
		Args:          command.Args,
		Dir:           command.Dir,
		Timestamp:     time.Now(),
		LastRunStatus: pass,
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

		err = b.Put([]byte(key), he.JSON())
		if err != nil {
			return err
		}

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

	// sort the entries by timestamp
	slices.SortFunc(entries, func(a, b HistoryEntry) int {
		if a.Timestamp.Before(b.Timestamp) {
			return 1
		}

		return -1
	})

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

			return strings.Contains(strings.ToLower(test.String()), strings.ToLower(input))
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

	// lookup the current module root based on working directory so that
	// we only run the last test in the current module and not the last global test.
	wd, err := os.Getwd()
	if err != nil {
		return HistoryEntry{}, fmt.Errorf("error getting working directory %w", err)
	}

	modRoot := lookupModuleRoot(wd)

	var lastCommand HistoryEntry
	err = file.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("history"))
		if b == nil {
			return nil
		}

		err = b.ForEach(func(k, v []byte) error {
			var he HistoryEntry
			he.Load(v)

			// only consider the last command in the current module
			if he.Dir != modRoot {
				return nil
			}

			if lastCommand.Timestamp.Before(he.Timestamp) {
				lastCommand = he
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("error iterating over history %w", err)
		}

		return nil
	})
	if err != nil {
		return HistoryEntry{}, fmt.Errorf("error viewing history file %w", err)
	}

	if lastCommand.Path == "" {
		return HistoryEntry{}, errors.New("no runs found for the current module")
	}

	return lastCommand, nil
}

func runHistoryEntry(he HistoryEntry) {
	var outputWriter io.Writer = os.Stdout
	if config.ColorizeOutput {
		var colorReader io.Reader
		colorReader, outputWriter = io.Pipe()
		go colorizeOutput(colorReader)
	}

	cmd := exec.Cmd{
		Path:   he.Path,
		Args:   he.Args,
		Dir:    he.Dir,
		Stdout: outputWriter,
		Stderr: os.Stderr,
	}

	fmt.Println("Running", cmd.Args, "@", cmd.Dir)

	var pass bool
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		pass = true
	// do nothing
	case errors.As(err, &exit):
	// do nothing
	default:
		panic(err)
	}

	logRunHistory(cmd, pass)
}
