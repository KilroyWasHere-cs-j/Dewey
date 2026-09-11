package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestManager returns a PluginManger pointed at a fresh temp directory
// instead of the real backend/plugins/ folder, so tests never touch or
// depend on the actual shipped plugin files.
func newTestManager(t *testing.T) *PluginManger {
	t.Helper()
	pm := newPluginManger()
	pm.dir = t.TempDir()
	return pm
}

// writePlugin drops a fixture .lua file into dir for loadPlugins to pick up.
func writePlugin(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writing fixture plugin %s: %v", name, err)
	}
}

func TestRegisterHookIsIdempotent(t *testing.T) {
	pm := newPluginManger()

	if pm.isRegistered("OnFilter") {
		t.Fatal("hook should not be registered before registerHook is called")
	}

	if err := pm.registerHook("OnFilter"); err != nil {
		t.Fatalf("registerHook: %v", err)
	}
	if !pm.isRegistered("OnFilter") {
		t.Fatal("hook should be registered after registerHook")
	}

	// Registering the same name again should be a harmless no-op, not an error.
	if err := pm.registerHook("OnFilter"); err != nil {
		t.Fatalf("re-registering an existing hook should not error, got: %v", err)
	}
}

func TestLoadPluginsAttachesOnlyRegisteredHooks(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	// OnDelete deliberately left unregistered.

	writePlugin(t, pm.dir, "filter.lua", `
Salience = 5
function WhoAmI() return Salience end
function OnFilter(entry) entry.Path = "routed/" .. entry.Path; return entry end
function OnDelete(entry) return entry end
`)

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
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
	pm.registerHook("OnFilter")

	writePlugin(t, pm.dir, "useless.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnSomethingElse(entry) return entry end
`)

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if len(pm.loadedPlugins) != 0 {
		t.Fatalf("expected a plugin implementing no registered hook to be skipped, got %d loaded", len(pm.loadedPlugins))
	}
}

func TestLoadPluginsDefaultsSalienceWhenNonNumeric(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")

	writePlugin(t, pm.dir, "badSalience.lua", `
function WhoAmI() return "not a number" end
function OnFilter(entry) return entry end
`)

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
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
	pm.registerHook("OnFilter")

	writePlugin(t, pm.dir, "broken.lua", `this is not valid lua {{{`)

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("one broken plugin file should not fail the whole load: %v", err)
	}
	if len(pm.loadedPlugins) != 0 {
		t.Fatalf("expected the broken plugin to be skipped, got %d loaded", len(pm.loadedPlugins))
	}
}

func TestLoadPluginsErrorsOnMissingDir(t *testing.T) {
	pm := newPluginManger()
	pm.dir = filepath.Join(t.TempDir(), "does-not-exist")

	if err := pm.loadPlugins(); err == nil {
		t.Fatal("expected an error when the plugin directory doesn't exist")
	}
}

func TestRunByHookErrorsOnUnregisteredHook(t *testing.T) {
	pm := newPluginManger()

	if _, err := pm.runByHook("OnNope", DBEntry{}); err == nil {
		t.Fatal("expected an error running a hook that was never registered")
	}
}

func TestRunByHookIsNoopWithNoAttachedPlugins(t *testing.T) {
	pm := newPluginManger()
	pm.registerHook("OnFilter")

	entry := DBEntry{Path: "original.txt"}
	got, err := pm.runByHook("OnFilter", entry)
	if err != nil {
		t.Fatalf("runByHook: %v", err)
	}
	if got != entry {
		t.Fatalf("expected entry unchanged when no plugin is attached, got %+v, want %+v", got, entry)
	}
}

func TestRunByHookRunsInSalienceOrder(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")

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

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}

	got, err := pm.runByHook("OnFilter", DBEntry{Path: "start"})
	if err != nil {
		t.Fatalf("runByHook: %v", err)
	}
	if want := "start-high-low"; got.Path != want {
		t.Fatalf("expected salience-ordered chain %q, got %q", want, got.Path)
	}
}

func TestRunByHookPluginErrorVetoes(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnDelete")

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

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}

	if _, err := pm.runByHook("OnDelete", DBEntry{Filename: "protected.txt"}); err == nil {
		t.Fatal("expected the plugin's error() call to propagate as a veto")
	}
	if _, err := pm.runByHook("OnDelete", DBEntry{Filename: "fine.txt"}); err != nil {
		t.Fatalf("unexpected veto for a non-protected file: %v", err)
	}
}

func TestRunByHookRoundTripsAllFields(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")

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

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}

	in := DBEntry{Filename: "fn", Act: "act", Hash: "hash", Path: "path", Meta: "meta", Barcode: sql.NullString{String: "bc", Valid: true}}
	got, err := pm.runByHook("OnFilter", in)
	if err != nil {
		t.Fatalf("runByHook: %v", err)
	}
	want := DBEntry{Filename: "fn-f", Act: "act-a", Hash: "hash-h", Path: "path-p", Meta: "meta-m", Barcode: sql.NullString{String: "bc-b", Valid: true}}
	if got != want {
		t.Fatalf("expected every field to round-trip, got %+v, want %+v", got, want)
	}
}

func TestListPluginsDoesNotPanicOnEmptyManager(t *testing.T) {
	pm := newPluginManger()
	pm.listPlugins()
}

// --- Runtime sandbox (issue #284) ---

// TestSandboxBlocksDangerousGlobalsAndStdlib confirms os/io are absent and
// none of the base-lib globals that could reconstruct that access (load,
// loadstring, dofile, loadfile, require, getfenv, setfenv) survive
// newSandboxedState's setup.
func TestSandboxBlocksDangerousGlobalsAndStdlib(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	writePlugin(t, pm.dir, "sandboxCheck.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    if dofile ~= nil then error("dofile should be nil") end
    if load ~= nil then error("load should be nil") end
    if loadstring ~= nil then error("loadstring should be nil") end
    if loadfile ~= nil then error("loadfile should be nil") end
    if require ~= nil then error("require should be nil") end
    if getfenv ~= nil then error("getfenv should be nil") end
    if setfenv ~= nil then error("setfenv should be nil") end
    if os ~= nil then error("os should not be open") end
    if io ~= nil then error("io should not be open") end
    return entry
end
`)
	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if _, err := pm.runByHook("OnFilter", DBEntry{}); err != nil {
		t.Fatalf("runByHook: %v", err)
	}
}

// --- Go-native capability API: http.get/http.post (issue #284) ---

// withUnblockedPluginHTTPClient swaps pluginHTTPClient's SSRF-blocking
// dialer out for the default one. httptest servers only bind to loopback —
// exactly what dialBlockingPrivateIPs refuses to reach — so wiring tests
// that need a real request/response round trip use this, while the
// blocking behavior itself is tested separately against the real client.
func withUnblockedPluginHTTPClient(t *testing.T) {
	t.Helper()
	orig := pluginHTTPClient
	pluginHTTPClient = &http.Client{}
	t.Cleanup(func() { pluginHTTPClient = orig })
}

func TestHTTPGetWiringRoundTrip(t *testing.T) {
	withUnblockedPluginHTTPClient(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from server"))
	}))
	defer srv.Close()

	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	writePlugin(t, pm.dir, "httpGet.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    local result = http.get("`+srv.URL+`")
    if not result:match("hello from server") then
        error("unexpected body: " .. result)
    end
    return entry
end
`)
	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if _, err := pm.runByHook("OnFilter", DBEntry{}); err != nil {
		t.Fatalf("runByHook: %v", err)
	}
}

func TestHTTPPostWiringRoundTrip(t *testing.T) {
	withUnblockedPluginHTTPClient(t)

	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.Write([]byte("posted"))
	}))
	defer srv.Close()

	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	writePlugin(t, pm.dir, "httpPost.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    local result = http.post("`+srv.URL+`", "hello-post-body")
    if not result:match("posted") then
        error("unexpected response: " .. result)
    end
    return entry
end
`)
	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if _, err := pm.runByHook("OnFilter", DBEntry{}); err != nil {
		t.Fatalf("runByHook: %v", err)
	}
	if !strings.Contains(receivedBody, "hello-post-body") {
		t.Fatalf("server did not receive posted body, got %q", receivedBody)
	}
}

// TestHTTPGetBlocksLoopback is the property that actually matters: with the
// real (blocking) pluginHTTPClient in place, a plugin's http.get to a
// loopback address (standing in for pod-internal MySQL/Prometheus, which
// are deliberately unpublished from the LAN per #200/#204) must fail, and
// that failure must surface as a normal, pcall-catchable Lua error rather
// than crashing the hook.
func TestHTTPGetBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("should never be reachable"))
	}))
	defer srv.Close()

	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	writePlugin(t, pm.dir, "httpBlocked.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    local ok, result = pcall(http.get, "`+srv.URL+`")
    if ok then
        error("expected http.get to a loopback address to fail, got: " .. tostring(result))
    end
    return entry
end
`)
	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if _, err := pm.runByHook("OnFilter", DBEntry{}); err != nil {
		t.Fatalf("runByHook (plugin should have caught the blocked call itself via pcall): %v", err)
	}
}

// --- Go-native capability API: files.read/files.write (issue #284) ---

func TestFilesReadWriteRoundTrip(t *testing.T) {
	// pluginScratchDir is a const, so this runs against the real relative
	// path (./plugin-scratch under backend/, since `go test` runs with the
	// package directory as cwd) and cleans up after itself.
	t.Cleanup(func() { os.RemoveAll(pluginScratchDir) })

	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	writePlugin(t, pm.dir, "filesRoundTrip.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    files.write("note.txt", "hello from plugin")
    local content = files.read("note.txt")
    if content ~= "hello from plugin" then
        error("round trip mismatch: " .. content)
    end
    return entry
end
`)
	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if _, err := pm.runByHook("OnFilter", DBEntry{}); err != nil {
		t.Fatalf("runByHook: %v", err)
	}
}

// TestReloadPluginsPreservesHookRegistration confirms a reload doesn't lose
// track of which hooks exist (issue #325's whole point is picking up
// filter changes without a restart, so a reload that forgets which hooks
// are registered — the way registerHook calls in main.go do it once at
// startup — would silently stop every plugin from attaching to anything).
func TestReloadPluginsPreservesHookRegistration(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")

	writePlugin(t, pm.dir, "filter.lua", `
Salience = 5
function WhoAmI() return Salience end
function OnFilter(entry) entry.Path = "routed/" .. entry.Path; return entry end
`)

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if got := len(pm.hooks["OnFilter"]); got != 1 {
		t.Fatalf("expected 1 plugin attached to OnFilter before reload, got %d", got)
	}

	if err := pm.reloadPlugins(); err != nil {
		t.Fatalf("reloadPlugins: %v", err)
	}

	if got := len(pm.hooks["OnFilter"]); got != 1 {
		t.Fatalf("expected 1 plugin still attached to OnFilter after reload, got %d", got)
	}
}

// TestReloadPluginsPicksUpNewlyAddedPlugin is the actual feature issue #325
// asks for: dropping a new plugin file in and reloading should attach it
// without restarting the server.
func TestReloadPluginsPicksUpNewlyAddedPlugin(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if got := len(pm.hooks["OnFilter"]); got != 0 {
		t.Fatalf("expected 0 plugins attached before any file exists, got %d", got)
	}

	writePlugin(t, pm.dir, "new.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry) return entry end
`)

	if err := pm.reloadPlugins(); err != nil {
		t.Fatalf("reloadPlugins: %v", err)
	}
	if got := len(pm.hooks["OnFilter"]); got != 1 {
		t.Fatalf("expected the newly-added plugin to be attached after reload, got %d", got)
	}
}

// TestReloadPluginsDropsRemovedPlugin confirms a reload's reset of
// loadedPlugins/hooks actually takes effect — a plugin file deleted from
// disk shouldn't still be attached after the next reload.
func TestReloadPluginsDropsRemovedPlugin(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")

	pluginPath := filepath.Join(pm.dir, "temp.lua")
	writePlugin(t, pm.dir, "temp.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry) return entry end
`)

	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if got := len(pm.hooks["OnFilter"]); got != 1 {
		t.Fatalf("expected 1 plugin attached before removal, got %d", got)
	}

	if err := os.Remove(pluginPath); err != nil {
		t.Fatalf("removing fixture plugin: %v", err)
	}

	if err := pm.reloadPlugins(); err != nil {
		t.Fatalf("reloadPlugins: %v", err)
	}
	if got := len(pm.hooks["OnFilter"]); got != 0 {
		t.Fatalf("expected the removed plugin to be gone after reload, got %d still attached", got)
	}
}

// TestFilesWriteBlocksTraversal is the property that actually matters:
// resolveStorePath rejects a "../" escape before any write happens, so
// unlike the round-trip test above, this never touches a real file.
func TestFilesWriteBlocksTraversal(t *testing.T) {
	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	writePlugin(t, pm.dir, "filesTraversal.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    local ok = pcall(files.write, "../escaped.txt", "should never land")
    if ok then
        error("expected a traversal write to fail")
    end
    return entry
end
`)
	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}
	if _, err := pm.runByHook("OnFilter", DBEntry{}); err != nil {
		t.Fatalf("runByHook: %v", err)
	}
}

// TestCallHookTimesOutOnInfiniteLoop proves the issue #404 fix actually
// interrupts a plugin that hangs via pure Lua execution — not just one
// stuck inside an http.get/http.post call, which already had its own
// separate, shorter timeout. Overrides the package-level multiplier to
// keep the test fast rather than waiting out the real configured value.
func TestCallHookTimesOutOnInfiniteLoop(t *testing.T) {
	original := pluginHookTimeoutMultiplier
	pluginHookTimeoutMultiplier = 1
	t.Cleanup(func() { pluginHookTimeoutMultiplier = original })

	pm := newTestManager(t)
	pm.registerHook("OnFilter")
	writePlugin(t, pm.dir, "hang.lua", `
Salience = 1
function WhoAmI() return Salience end
function OnFilter(entry)
    while true do end
    return entry
end
`)
	if err := pm.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins: %v", err)
	}

	start := time.Now()
	_, err := pm.runByHook("OnFilter", DBEntry{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the infinite loop to be interrupted, got a nil error")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("hook call took %v to return — the timeout doesn't appear to be interrupting execution", elapsed)
	}
}
