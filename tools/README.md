# Tools

Development tool dependencies for `go-ultimate`, kept separate from the main module to avoid adding them to the main module's dependency graph.

Use the tools with:

```sh
go tool -modfile=tools/go.mod <tool-package>
```
