## go-test

go-test is a CLI to find and execute tests in a Go project. It's a wrapper
around `go test` with some additional features. Such as the ability to find and
execute subtests within table driven tests. You can also rerun your last test and track your test execution history.

### Installation

```bash
go install github.com/mordfustang21/go-test@latest
```

Notable features:
- Find and execute tests in a Go project INCLUDING SUBTESTS
- Easily test for coverage and then launch in a browser
- Test execution history with re-run capability
- Memory and CPU profiling WITH Flamegraph support 🔥
