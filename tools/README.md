# Tools

This directory includes tools for `go-ultimate`.
It is kept separate from the main module to avoid adding them to the main module's dependency graph.

## c64ctl

`c64ctl` exposes `go-ultimate` capabilities to command line.

Install with `go install` or run directly without installing:

```
go run github.com/c64uploader/go-ultimate/tools/c64ctl@latest
```


## Development tools

Use the development tools by running:

```sh
go tool -modfile=tools/go.mod <tool-package>
```

Current dev tools

* `golangci-lint` - used by `make lint` target.

