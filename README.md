## gotest

Gotest is a command-line interface (CLI) tool designed to locate and run tests in a Go project. It serves as a wrapper around the `go test` command, offering extra functionalities.
These include the capability to identify and execute subtests within table-driven tests, the option to rerun your most recent test, and the ability to maintain a history of your test executions.

### Installation
```bash
go install github.com/MordFustang21/gotest@latest
```

### Usage
```bash
# Run all tests in the current directory
❯ gotest

# Find a specific test
❯ gotest -s
Use the arrow keys to navigate: ↓ ↑ → ←  and / toggles search
Select a subtest
  > Test_loadConfig
    Test_loadConfig/basic
    Test_loadConfig/basic with comment
    Test_loadConfig/basic unknown field
↓   Test_packageFromPathAndMod

# Run test with debugger
❯ gotest -s -d

# Run a benchmark
❯ gotest -b

# Rerun the last test run
❯ gotest -r

# Run a test with coverage
❯ gotest -cover

# Run a test with coverage and open in browser
❯ gotest -cpu
```

Notable features:
- Find and execute tests in a Go project INCLUDING SUBTESTS AND TABLE-DRIVEN TESTS
- Memory and CPU profiling WITH Flamegraph support 🔥
- Easily test for coverage and then launch in a browser
- Test execution history with re-run capability
