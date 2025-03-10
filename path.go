package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

func testToPathAndRoot(t Test) (path string, modRoot string) {
	path = t.File

	// if this is a file and not a directory, use the directory of the file
	if filepath.Ext(t.File) != "" {
		path = filepath.Dir(t.File)
	}

	modRoot = lookupModuleRoot(path)
	if modRoot == "" {
		panic("could not find module root")
	}

	// convert path to a module path
	path, _ = filepath.Rel(modRoot, path)
	path = "./" + path

	// in the event the directory is the root of the module, and there isn't a named test specified we need to add an extra ".." to
	// tell go test to recursively run all tests
	if path == "./." && t.Name == "" {
		path += ".."
	}

	return path, modRoot
}

func lookupModuleRoot(path string) string {
	// start at end and work backwards to find the go.mod file
	for {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			return path
		}

		path = filepath.Dir(path)

		if path == "/" {
			break
		}
	}

	return ""
}

func resolvePackage(path, modRoot string) string {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}", path)
	cmd.Dir = modRoot
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		panic(err)
	}

	return string(bytes.TrimSpace(out))
}

func packageFromPathAndMod(path, modRoot string) string {
	out, err := filepath.Rel(modRoot, path)
	if err != nil {
		return ""
	}

	return out
}
