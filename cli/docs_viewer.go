package main

import (
	"embed"
	"fmt"
)

// docsSource holds one loaded doc's raw markdown content, or the error hit
// while loading it. Mirrors metricsSnapshot's shape (metrics_fetch.go) so
// the dashboard can follow the same State/error-branch pattern the metrics
// view (issue #348) already uses.
type docsSource struct {
	content string
	err     error
}

// embeddedDocs bakes the staged markdown copies into the dewey-cli binary,
// so `dewey-cli docs` works regardless of where the binary ends up running
// (package.sh copies dewey-cli into a separate bundle dir, away from the
// repo — a path read at runtime wouldn't find anything there).
//
// go:embed can't reach files outside its own module directory, so the real
// source docs (repo-root README.md, frontend/doctooladmin/README.md) are
// staged into this folder by package.sh right before the build. Running a
// bare `go build .` here without package.sh having run first will fail to
// compile until embedded_docs/ has been populated at least once.
//
//go:embed embedded_docs/readme.md embedded_docs/admin_readme.md
var embeddedDocs embed.FS

// docPaths lists the docs the viewer can show, keyed by the name a user
// would pass on the command line (e.g. `dewey-cli docs readme`), mapped to
// their staged path inside embeddedDocs.
var docPaths = map[string]string{
	"readme": "embedded_docs/readme.md",
	"admin":  "embedded_docs/admin_readme.md",
}

// loadDoc reads the named doc's embedded markdown source.
func loadDoc(name string) docsSource {
	path, ok := docPaths[name]
	if !ok {
		return docsSource{err: fmt.Errorf("unknown doc %q (want one of: readme, admin)", name)}
	}
	data, err := embeddedDocs.ReadFile(path)
	if err != nil {
		return docsSource{err: err}
	}
	return docsSource{content: string(data)}
}
