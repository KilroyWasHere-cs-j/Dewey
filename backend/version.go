package main

// GitBranch is the git branch this binary was built from. It can't have a
// meaningful default baked into consts.go since it's only known at build
// time, so it's injected via -ldflags "-X main.GitBranch=<branch>" in the
// Dockerfile (see backend/Dockerfile and deploy.sh/package.sh, which pass
// the branch as a --build-arg). Falls back to "unknown" for local
// `go run`/`go build` invocations that skip the ldflag.
var GitBranch = "unknown"
