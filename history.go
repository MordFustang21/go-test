package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

const historyFile = ".go-test-history"

// HistoryEntry is a single entry in the history file.
type HistoryEntry struct {
	Path string
	Args []string
	Dir  string
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

func getHistoryFile() *os.File {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	p := filepath.Join(home, historyFile)

	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}

	return f
}

func logRunHistory(command exec.Cmd) {
	// get current working directory and make it relative to the command's directory
	// this is so we can store the command and execute from any directory
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	rootPath := filepath.Join(wd, command.Dir)

	he := HistoryEntry{
		Path: command.Path,
		Args: command.Args,
		Dir:  rootPath,
	}

	file := getHistoryFile()
	defer file.Close()

	// write the command to the file
	_, err = file.Write(append(he.JSON(), '\n'))
	if err != nil {
		panic(err)
	}
}

func selectHistory() HistoryEntry {
	file := getHistoryFile()
	defer file.Close()

	return HistoryEntry{}
}

func getLastCommand() HistoryEntry {
	file := getHistoryFile()
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	var he HistoryEntry
	he.Load([]byte(lastLine))

	return he
}

func runHistoryEntry(he HistoryEntry) {
	cmd := exec.Cmd{
		Path:   he.Path,
		Args:   he.Args,
		Dir:    he.Dir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}
