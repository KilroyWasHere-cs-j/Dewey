package main

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestManager returns a PluginManger pointed at a fresh temp directory
// instead of the real backend/plugins/ folder, so tests never touch or
// depend on the actual shipped plugin files.
func newTestManager(t *testing.T) *PluginManger {
	t.Helper()
	pm := NewPluginManger()
	pm.dir = t.TempDir()
	return pm
}

// writePlugin drops a fixture .lua file into dir for LoadPlugins to pick up.
func writePlugin(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writing fixture plugin %s: %v", name, err)
	}
}

func TestRegisterHookIsIdempotent(t *testing.T) {
	pm := NewPluginManger()

	if pm.IsRegistered("OnFilter") {
		t.Fatal("hook should not be registered before RegisterHook is called")
	}

	if err := pm.RegisterHook("OnFilter"); err != nil {
		t.Fatalf("RegisterHook: %v", err)
	}
	if !pm.IsRegistered("OnFilter") {
		t.Fatal("hook should be registered after RegisterHook")
	}

	// Registering the same name again should be a harmless no-op, not an error.
	if err := pm.RegisterHook("OnFilter"); err != nil {
		t.Fatalf("re-registering an existing hook should not error, got: %v", err)
	}
}

func TestLoadPluginsAttachesOnlyRegisteredHooks(t *testing.T) {
	pm := newTestManager(t)
	pm.RegisterHook("OnFilter")
	// OnDelete deliberately left unregistered.

	writePlugin(t, pm.dir, "filter.lua", `
Salience = 5
function WhoAmI() return Salience end
function OnFilter(entry) entry.Path = "routed/" .. entry.Path; return entry end
function OnDelete(entry) return entry end
`)

	if err := pm.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	if got := len(pm.hooks["OnFilter"]); got != 1 {
		t.Fatalf("expected 1 plugin attached to OnFilter, got %d", got)
	}
	if got := len(pm.hooks["OnDelete"]); got != 0 {
		t.Fatalf("OnDelete was never registered, expected 0 plugins attached, got %d", got)
	}
}

func TestLoadPluginsSkipsFileImplementingNoRegisteredHook(t *testing.T) {
	pm := newTestManager(t)
	pm.RegisterHook("OnFilter")

	writePlugin(t, pm.dir, "useless.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnSomethingElse(entry) return entry end
`)

	if err := pm.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	if len(pm.loadedPlugins) != 0 {
		t.Fatalf("expected a plugin implementing no registered hook to be skipped, got %d loaded", len(pm.loadedPlugins))
	}
}

func TestLoadPluginsDefaultsSalienceWhenNonNumeric(t *testing.T) {
	pm := newTestManager(t)
	pm.RegisterHook("OnFilter")

	writePlugin(t, pm.dir, "badSalience.lua", `
function WhoAmI() return "not a number" end
function OnFilter(entry) return entry end
`)

	if err := pm.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	if len(pm.loadedPlugins) != 1 {
		t.Fatalf("expected plugin to still load with a default salience, got %d loaded", len(pm.loadedPlugins))
	}
	if pm.loadedPlugins[0].salience != 0 {
		t.Fatalf("expected default salience 0, got %d", pm.loadedPlugins[0].salience)
	}
}

func TestLoadPluginsSkipsFileWithSyntaxError(t *testing.T) {
	pm := newTestManager(t)
	pm.RegisterHook("OnFilter")

	writePlugin(t, pm.dir, "broken.lua", `this is not valid lua {{{`)

	if err := pm.LoadPlugins(); err != nil {
		t.Fatalf("one broken plugin file should not fail the whole load: %v", err)
	}
	if len(pm.loadedPlugins) != 0 {
		t.Fatalf("expected the broken plugin to be skipped, got %d loaded", len(pm.loadedPlugins))
	}
}

func TestLoadPluginsErrorsOnMissingDir(t *testing.T) {
	pm := NewPluginManger()
	pm.dir = filepath.Join(t.TempDir(), "does-not-exist")

	if err := pm.LoadPlugins(); err == nil {
		t.Fatal("expected an error when the plugin directory doesn't exist")
	}
}

func TestRunByHookErrorsOnUnregisteredHook(t *testing.T) {
	pm := NewPluginManger()

	if _, err := pm.RunByHook("OnNope", DBEntry{}); err == nil {
		t.Fatal("expected an error running a hook that was never registered")
	}
}

func TestRunByHookIsNoopWithNoAttachedPlugins(t *testing.T) {
	pm := NewPluginManger()
	pm.RegisterHook("OnFilter")

	entry := DBEntry{Path: "original.txt"}
	got, err := pm.RunByHook("OnFilter", entry)
	if err != nil {
		t.Fatalf("RunByHook: %v", err)
	}
	if got != entry {
		t.Fatalf("expected entry unchanged when no plugin is attached, got %+v, want %+v", got, entry)
	}
}

func TestRunByHookRunsInSalienceOrder(t *testing.T) {
	pm := newTestManager(t)
	pm.RegisterHook("OnFilter")

	// Higher salience runs first, so "high" should be appended before "low".
	writePlugin(t, pm.dir, "low.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry) entry.Path = entry.Path .. "-low"; return entry end
`)
	writePlugin(t, pm.dir, "high.lua", `
Salience = 10
function WhoAmI() return Salience end
function OnFilter(entry) entry.Path = entry.Path .. "-high"; return entry end
`)

	if err := pm.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	got, err := pm.RunByHook("OnFilter", DBEntry{Path: "start"})
	if err != nil {
		t.Fatalf("RunByHook: %v", err)
	}
	if want := "start-high-low"; got.Path != want {
		t.Fatalf("expected salience-ordered chain %q, got %q", want, got.Path)
	}
}

func TestRunByHookPluginErrorVetoes(t *testing.T) {
	pm := newTestManager(t)
	pm.RegisterHook("OnDelete")

	writePlugin(t, pm.dir, "veto.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnDelete(entry)
    if entry.Filename == "protected.txt" then
        error("blocked")
    end
    return entry
end
`)

	if err := pm.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	if _, err := pm.RunByHook("OnDelete", DBEntry{Filename: "protected.txt"}); err == nil {
		t.Fatal("expected the plugin's error() call to propagate as a veto")
	}
	if _, err := pm.RunByHook("OnDelete", DBEntry{Filename: "fine.txt"}); err != nil {
		t.Fatalf("unexpected veto for a non-protected file: %v", err)
	}
}

func TestRunByHookRoundTripsAllFields(t *testing.T) {
	pm := newTestManager(t)
	pm.RegisterHook("OnFilter")

	writePlugin(t, pm.dir, "mutateAll.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    entry.Filename = entry.Filename .. "-f"
    entry.Act = entry.Act .. "-a"
    entry.Hash = entry.Hash .. "-h"
    entry.Path = entry.Path .. "-p"
    entry.Meta = entry.Meta .. "-m"
    entry.Barcode = entry.Barcode .. "-b"
    return entry
end
`)

	if err := pm.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	in := DBEntry{Filename: "fn", Act: "act", Hash: "hash", Path: "path", Meta: "meta", Barcode: "bc"}
	got, err := pm.RunByHook("OnFilter", in)
	if err != nil {
		t.Fatalf("RunByHook: %v", err)
	}
	want := DBEntry{Filename: "fn-f", Act: "act-a", Hash: "hash-h", Path: "path-p", Meta: "meta-m", Barcode: "bc-b"}
	if got != want {
		t.Fatalf("expected every field to round-trip, got %+v, want %+v", got, want)
	}
}

func TestListPluginsDoesNotPanicOnEmptyManager(t *testing.T) {
	pm := NewPluginManger()
	pm.ListPlugins()
}
