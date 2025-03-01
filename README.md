## gotest

Gotest is a command-line interface (CLI) tool designed to locate and run tests in a Go project. It serves as a wrapper around the `go test` command, offering extra functionalities.
These include the capability to identify and execute subtests within table-driven tests, the option to rerun your most recent test, and the ability to maintain a history of your test executions.

### Installation
```bash
go install github.com/MordFustang21/gotest@latest
```

Notable features:
- Find and execute tests in a Go project INCLUDING SUBTESTS AND TABLE-DRIVEN TESTS
- Memory and CPU profiling WITH Flamegraph support 🔥
- Easily test for coverage and then launch in a browser
- Test execution history with re-run capability
