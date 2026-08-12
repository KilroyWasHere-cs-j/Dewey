package main

import (
	"os"
	"testing"
)

// TestMain loads config.json before any test runs — main() normally does
// this first thing, but go test never calls main(), so without this every
// test would see pluginDir/pluginScratchDir/etc. at their zero values
// instead of the real config (issue #256).
func TestMain(m *testing.M) {
	load()
	os.Exit(m.Run())
}
