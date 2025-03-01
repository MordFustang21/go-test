package flamegraph

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
)

// flamegraph.pl is a perl script that generates a flamegraph svg from a folded stack trace.
//
//go:embed flamegraph.pl
var flamegraphScript string

// stackcollapse-go.pl is a perl script that converts a raw pprof file to a folded stack trace.
//
//go:embed stackcollapse-go.pl
var collapseScript string

// GenerateFlamegraph takes in a pprof file and returns a flamegraph svg.
func GenerateFlamegraph(file string) ([]byte, error) {
	// Profiles are in a zipped binary format that needs to be converted to a raw format.
	rawPPROF, err := profileToRaw(file)
	if err != nil {
		fmt.Println("Error converting profile to raw:", err)
		return nil, err
	}

	// Fold the raw pprof data into single lines.
	foldedRaw, err := foldRaw(rawPPROF)
	if err != nil {
		fmt.Println("Error folding raw:", err)
		return nil, err
	}

	// Convert the folded stack trace to a flamegraph svg.
	svgBytes, err := foldedToFlamegraph(foldedRaw)
	if err != nil {
		return nil, fmt.Errorf("error generating flamegraph svg: %w", err)
	}

	return svgBytes, nil
}

func foldedToFlamegraph(folded []byte) ([]byte, error) {
	// Run the flamegraph.pl script
	cmd := exec.Command("perl", "-e", flamegraphScript)

	// Create a pipe to write the profile input to the flamegraph.pl script
	cmd.Stdin = bytes.NewBuffer(folded)

	// Write output svg to a buffer
	var output bytes.Buffer
	cmd.Stdout = &output

	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error starting flamegraph.pl:", err)
		return nil, err
	}

	return output.Bytes(), nil
}

// profileToRaw converts a pprof file to a raw format that can be understood by stackcollapse-go.pl.
func profileToRaw(file string) ([]byte, error) {
	var raw bytes.Buffer
	cmd := exec.Command("go", "tool", "pprof", "-raw", file)
	cmd.Stdout = &raw
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error starting pprof:", err)
		return nil, err
	}

	return raw.Bytes(), nil
}

// foldRaw converts a raw pprof file to a folded stack trace.
func foldRaw(pprofData []byte) ([]byte, error) {
	cmd := exec.Command("perl", "-e", collapseScript)
	cmd.Stdin = bytes.NewBuffer(pprofData)

	var folded bytes.Buffer
	cmd.Stdout = &folded
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error starting stackcollapse-go.pl:", err)
		return nil, err
	}

	return folded.Bytes(), nil
}

func ServeFlamegraph(data []byte) error {
	ctx, cancel := context.WithCancel(context.Background())

	// Get a random port to serve the flamegraph on.
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, err := w.Write(data)
		if err != nil {
			fmt.Println("Error writing flamegraph:", err)
		}

		// Cancel the context to cleanup.
		cancel()
	})

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("couldn't create net listener: %w", err)
	}

	srvr := &http.Server{Addr: ":0"}

	go func() {
		err := srvr.Serve(listener)
		switch {
		case errors.Is(err, http.ErrServerClosed):
		default:
			fmt.Println("Error serving flamegraph:", err)
		}
	}()

	// Launch the browser to view the flamegraph.
	cmd := exec.Command("open", "http://"+listener.Addr().String())
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error starting browser:", err)
	}

	<-ctx.Done()
	srvr.Close()

	return nil
}
