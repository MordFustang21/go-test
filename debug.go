package main

import (
	"fmt"
	"os"
	"os/exec"
)

func debugTest(t Test, path, modRoot string) (exec.Cmd, bool) {
	p, err := exec.LookPath("dlv")
	if err != nil {
		panic(err)
	}

	// Create a temp file to set breakpoints and tell dlv to continue.
	tempFile, err := os.CreateTemp("", "go-test_*")
	if err != nil {
		panic(err)
	}

  // Attempt cleanup when no longer in use.
  defer func ()  {
    err = os.Remove(tempFile.Name())
    if err != nil {
      panic(err)
    }
  }()

	tempFile.Write([]byte("b " + fmt.Sprintf("%s:%d", packageFromPathAndMod(t.FilePath, modRoot), t.LineNumber) + "\n"))
	tempFile.Write([]byte("c\n"))
	tempFile.Close()

	cmd := exec.Cmd{
		Path:   p,
		Env:    os.Environ(),
		Args:   []string{"dlv", "test", "--init", tempFile.Name(), resolvePackage(path, modRoot), "--", "-test.run", t.Name},
		Dir:    modRoot,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	err = cmd.Run()
	if err != nil {
		panic(err)
	}

	return cmd, true
}
