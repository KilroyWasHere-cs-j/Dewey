package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidatorAcceptsRealPlugins is the actual acceptance bar for the
// static validator: every real shipped plugin must pass unmodified. A
// false positive here (a real plugin getting rejected) is worse than the
// validator being slightly permissive.
func TestValidatorAcceptsRealPlugins(t *testing.T) {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		t.Fatalf("reading %s: %v", pluginDir, err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one real shipped plugin")
	}
	for _, entry := range entries {
		src, err := os.ReadFile(filepath.Join(pluginDir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		if err := validatePluginSource(src, entry.Name()); err != nil {
			t.Errorf("real shipped plugin %s was rejected: %v", entry.Name(), err)
		}
	}
}

// TestValidatorEndToEndLoadsRealPlugins confirms loadPlugins itself (not
// just validatePluginSource directly) actually loads all four real
// plugins — i.e. the wiring in loadPlugins doesn't silently skip anything.
func TestValidatorEndToEndLoadsRealPlugins(t *testing.T) {
	pm := newPluginManger()
	pm.dir = pluginDir
	pm.registerHook("OnFilter")
	pm.registerHook("OnDelete")
	pm.registerHook("OnUpload")
	pm.registerHook("OnTick")

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if len(pm.loadedPlugins) != 4 {
		t.Fatalf("expected all 4 real shipped plugins to load, got %d", len(pm.loadedPlugins))
	}
}

// TestValidatorRejectsAdversarialPlugins is the security property that
// actually matters: each of these must be rejected pre-execution, without
// ever creating a Lua state to run them in.
func TestValidatorRejectsAdversarialPlugins(t *testing.T) {
	cases := map[string]string{
		"string-reconstructed os.execute": `
function WhoAmI() return 1 end
function OnFilter(entry)
    local o = os
    o["ex" .. "ecute"]("echo pwned")
    return entry
end
`,
		"_G table escape": `
function WhoAmI() return 1 end
function OnFilter(entry)
    local g = _G
    g.os.execute("echo pwned")
    return entry
end
`,
		"bare getfenv reference": `
function WhoAmI() return 1 end
function OnFilter(entry)
    local f = getfenv
    return entry
end
`,
		"direct os reference": `
function WhoAmI() return 1 end
function OnFilter(entry)
    os.exit(1)
    return entry
end
`,
		"unlisted dot-call on string": `
function WhoAmI() return 1 end
function OnFilter(entry)
    entry.Path = string.format("%s", entry.Path)
    return entry
end
`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validatePluginSource([]byte(src), name); err == nil {
				t.Fatalf("expected %q to be rejected by static validation, but it passed", name)
			}
		})
	}
}
