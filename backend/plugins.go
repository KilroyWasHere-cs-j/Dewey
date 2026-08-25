package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// Plugin struct
type Plugin struct {
	salience int
	name     string
	// pool is this plugin's own LState pool, loaded only with that plugin's
	// file — this ensures Begin() resolves to the correct plugin's function
	// rather than whichever plugin happened to load last.
	pool *sync.Pool
}

// Plugin manager
type PluginManger struct {
	// registeredHooks is a set (empty-struct values cost nothing) of hook
	// names that have been declared — membership is a single map lookup
	// instead of scanning a slice. A hook must be registered before
	// loadPlugins runs, since loadPlugins only checks plugin files against
	// already-registered names.
	registeredHooks map[string]struct{}
	// hooks maps a registered hook name to every plugin that implements it
	// (i.e. defines a global function with that exact name), sorted by
	// salience descending. This is the actual dispatch table runByHook uses.
	hooks         map[string][]Plugin
	loadedPlugins []Plugin
	// dir is where loadPlugins reads plugin files from. Defaults to the
	// package-level pluginDir const; tests override it with a temp
	// directory so they don't have to touch the real plugins/ folder.
	dir          string
	pluginHashes map[string]string
}

// newPluginManger constructs an empty PluginManger with no plugins loaded
// yet — callers still need to call loadPlugins to populate it.
func newPluginManger() *PluginManger {
	return &PluginManger{
		registeredHooks: make(map[string]struct{}),
		hooks:           make(map[string][]Plugin),
		loadedPlugins:   make([]Plugin, 0),
		dir:             pluginDir,
		pluginHashes:    make(map[string]string),
	}
}

// --- Go-native capability API (issue #284) ---
//
// Plugins get no os/io access (see newSandboxedState below), so this is the
// only sanctioned way for one to reach the network or the filesystem. Built
// bottom-up: isDisallowedPluginIP/dialBlockingPrivateIPs/pluginHTTPClient
// enforce the SSRF restriction for the network side, resolvePluginScratchPath
// enforces the path-traversal restriction for the file side, httpGet/
// httpPost/fileRead/fileWrite are the Go-side implementations, and the
// luaHTTPGet/luaHTTPPost/luaFileRead/luaFileWrite adapters let
// newSandboxedState register them as the "http" and "files" global tables.

// isDisallowedPluginIP reports whether ip is a loopback, private, link-local,
// or unspecified address — the ranges a plugin's HTTP client must never
// reach. MySQL and Prometheus are deliberately unpublished from the LAN
// (issues #200, #204) and have no auth of their own, on the assumption
// they're unreachable from outside the pod's network namespace; an
// unrestricted plugin HTTP client would be a direct SSRF path back into
// both.
func isDisallowedPluginIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// dialBlockingPrivateIPs resolves addr's host itself and checks every
// candidate against isDisallowedPluginIP before dialing, then dials that
// exact IP rather than the original hostname. Checking only the URL's
// hostname up front (rather than the address actually being connected to)
// would leave a DNS-rebinding gap: a hostname that resolves to a public IP
// at check time but a pod-internal one at dial time. Resolving once and
// dialing the specific IP we vetted closes that gap.
func dialBlockingPrivateIPs(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}

	var dialer net.Dialer
	var lastErr error
	for _, ip := range ips {
		if isDisallowedPluginIP(ip) {
			lastErr = fmt.Errorf("refusing to dial disallowed address %s (resolved from %s)", ip, host)
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no addresses resolved for %s", host)
	}
	return nil, lastErr
}

// pluginHTTPClient is the only HTTP client plugin code can reach, via
// http.get/http.post. Its Transport dials through dialBlockingPrivateIPs
// instead of the default dialer.
var pluginHTTPClient = &http.Client{
	Transport: &http.Transport{DialContext: dialBlockingPrivateIPs},
}

// httpGet is the Go-side implementation behind the Lua "http.get" global —
// deliberately going through pluginHTTPClient rather than http.Get, so
// every request a plugin makes is subject to the SSRF check above.
func (pm *PluginManger) httpGet(url string) (string, error) {
	resp, err := pluginHTTPClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// httpPost is the Go-side implementation behind the Lua "http.post" global.
// Same pluginHTTPClient as httpGet, same SSRF check applied.
func (pm *PluginManger) httpPost(url string, body string) (string, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	resp, err := pluginHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

// luaHTTPGet and luaHTTPPost adapt httpGet/httpPost to gopher-lua's
// LGFunction signature: pull arguments off the Lua stack, call the Go-side
// implementation (which goes through pluginHTTPClient's SSRF-safe dialer),
// and push the result back. A Go error becomes a raised Lua error rather
// than a second return value, so a plugin can catch it with pcall the same
// way it would any other Lua error.
func (pm *PluginManger) luaHTTPGet(L *lua.LState) int {
	url := L.CheckString(1)
	body, err := pm.httpGet(url)
	if err != nil {
		L.RaiseError("http.get: %s", err.Error())
		return 0
	}
	L.Push(lua.LString(body))
	return 1
}

func (pm *PluginManger) luaHTTPPost(L *lua.LState) int {
	url := L.CheckString(1)
	body := L.CheckString(2)
	respBody, err := pm.httpPost(url, body)
	if err != nil {
		L.RaiseError("http.post: %s", err.Error())
		return 0
	}
	L.Push(lua.LString(respBody))
	return 1
}

// fileRead and fileWrite are the Go-side implementations behind the Lua
// "files.read"/"files.write" globals — the only filesystem access a plugin
// gets now that os/io are gone from the sandbox. Both confine name to
// pluginScratchDir via resolveStorePath, the same traversal check
// filemanager.go already uses to keep uploads within fileSystemBaseDir —
// a plain filepath.Join+Clean wouldn't catch a "../" escape on its own.
func (pm *PluginManger) fileRead(name string) (string, error) {
	path, err := resolveStorePath(pluginScratchDir, name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (pm *PluginManger) fileWrite(name string, data string) error {
	path, err := resolveStorePath(pluginScratchDir, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o600)
}

// luaFileRead and luaFileWrite adapt fileRead/fileWrite to gopher-lua's
// LGFunction signature, the same pattern as luaHTTPGet/luaHTTPPost above.
func (pm *PluginManger) luaFileRead(L *lua.LState) int {
	filePath := L.CheckString(1)
	fileContent, err := pm.fileRead(filePath)
	if err != nil {
		L.RaiseError("files.read: %s", err.Error())
		return 0
	}
	L.Push(lua.LString(fileContent))
	return 1
}

func (pm *PluginManger) luaFileWrite(L *lua.LState) int {
	filePath := L.CheckString(1)
	fileContent := L.CheckString(2)
	err := pm.fileWrite(filePath, fileContent)
	if err != nil {
		L.RaiseError("files.write: %s", err.Error())
		return 0
	}
	return 1
}

// dangerousBaseGlobals are registered by lua.OpenBase alongside the safe
// base functions (print, pairs, type, pcall, error, ...) — OpenBase bundles
// them all into one unexported map, so they can't be excluded at open time
// and have to be stripped afterward instead. Left in place, any of these
// let a plugin construct/execute arbitrary code from a string, read/execute
// arbitrary files, or swap a function's environment table.
var dangerousBaseGlobals = []string{"load", "loadstring", "dofile", "loadfile", "require", "getfenv", "setfenv"}

// newSandboxedState returns an LState with only base, string, and table
// opened (math is intentionally left out — no current plugin uses it), and
// the dangerous base globals above nil'd out. Every lua.NewState() call in
// this file must go through here instead of calling it directly, so a
// plugin file never gets os/io access or a way to construct code from a
// string (issue #284, superseding #199).
//
// It's a method (not a free function) so it can register pm's Go-native
// capability functions — http.get/http.post and files.read/files.write
// above — as the only sanctioned way for a plugin to reach the network or
// filesystem, in place of the os/io access it no longer has.
func (pm *PluginManger) newSandboxedState() *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	lua.OpenBase(L)
	lua.OpenString(L)
	lua.OpenTable(L)
	for _, name := range dangerousBaseGlobals {
		L.SetGlobal(name, lua.LNil)
	}

	httpTbl := L.NewTable()
	L.SetField(httpTbl, "get", L.NewFunction(pm.luaHTTPGet))
	L.SetField(httpTbl, "post", L.NewFunction(pm.luaHTTPPost))
	L.SetGlobal("http", httpTbl)

	filesTbl := L.NewTable()
	L.SetField(filesTbl, "read", L.NewFunction(pm.luaFileRead))
	L.SetField(filesTbl, "write", L.NewFunction(pm.luaFileWrite))
	L.SetGlobal("files", filesTbl)

	return L
}

// Plugin loader
func (pm *PluginManger) loadPlugins() error {
	entries, err := os.ReadDir(pm.dir)
	if err != nil {
		Warn("Unable to read plugin directory: " + err.Error())
		return err
	}

	for _, entry := range entries {
		pluginPath := filepath.Join(pm.dir, entry.Name())

		// Static validation (issue #284) runs before any Lua state exists
		// for this file — a malicious plugin's top-level code executes
		// immediately on DoFile below, before any hook function is ever
		// called, so this is the only thing that catches it pre-execution.
		src, err := os.ReadFile(pluginPath)
		if err != nil {
			Warn("Unable to read plugin " + entry.Name() + ": " + err.Error())
			continue
		}
		if err := validatePluginSource(src, entry.Name()); err != nil {
			Warn("Plugin failed static validation: " + err.Error())
			continue
		}

		// Fresh state per plugin file for discovery, closed at the end of
		// this iteration (issue #216) — a single state reused across every
		// DoFile call let Lua globals (WhoAmI, etc.) persist between files,
		// so a plugin missing WhoAmI silently inherited the previous
		// plugin's type/salience instead of failing to classify.
		L := pm.newSandboxedState()
		if err := L.DoFile(pluginPath); err != nil {
			Warn("Unable to load plugin " + entry.Name() + ": " + err.Error())
			L.Close()
			continue
		}

		// Identify the salience of the plugin. WhoAmI now reports salience
		// only — attachment to a hook comes from the plugin's declared
		// name/function, not from this call.
		whoAmIFunc := L.GetGlobal("WhoAmI")
		err = L.CallByParam(lua.P{
			Fn:      whoAmIFunc,
			NRet:    1, // Number of return values
			Protect: true,
		})
		if err != nil {
			// A failed call (e.g. WhoAmI missing entirely) pushes nothing
			// onto the stack — the unconditional Get/Pop(1) below would
			// underflow and panic the whole process, not just this plugin,
			// now that each file gets its own fresh state instead of
			// silently inheriting the previous plugin's WhoAmI (issue #216).
			Warn("Unable to call WhoAmI: " + err.Error())
			L.Close()
			continue
		}

		salienceLV := L.Get(-1)
		L.Pop(1) // clean WhoAmI's return value off the stack

		sVal, ok := salienceLV.(lua.LNumber)
		if !ok {
			Warn(fmt.Sprintf("plugin %s: salience must be a number, got %s", entry.Name(), salienceLV.Type()))
			sVal = 0 // Default to 0 if not a number
		}
		salience := int(sVal)

		// A plugin attaches to a hook by defining a global function named
		// after it — check every registered hook name against this file's
		// globals while its discovery state is still open. A plugin can
		// implement more than one hook (e.g. both OnFilter and OnDelete).
		var implementedHooks []string
		for hookName := range pm.registeredHooks {
			if fn := L.GetGlobal(hookName); fn.Type() == lua.LTFunction {
				implementedHooks = append(implementedHooks, hookName)
			}
		}
		L.Close()

		if len(implementedHooks) == 0 {
			Warn(fmt.Sprintf("plugin %s does not implement any registered hook, skipping", entry.Name()))
			continue
		}

		// Build a per-plugin state pool. Capturing pluginFile by value in the
		// closure avoids the loop variable capture bug.
		pluginFile := filepath.Join(pm.dir, entry.Name())
		pool := &sync.Pool{
			New: func() any {
				state := pm.newSandboxedState()
				if err := state.DoFile(pluginFile); err != nil {
					Warn("pool: failed to load " + pluginFile + ": " + err.Error())
				}
				return state
			},
		}

		plugin := Plugin{name: entry.Name(), salience: salience, pool: pool}
		pm.loadedPlugins = append(pm.loadedPlugins, plugin)
		for _, hookName := range implementedHooks {
			pm.hooks[hookName] = append(pm.hooks[hookName], plugin)
		}
	}

	// Highest salience runs first, both for the flat list and within each
	// hook's own bucket.
	bySalienceDesc := func(plugins []Plugin) func(i, j int) bool {
		return func(i, j int) bool { return plugins[i].salience > plugins[j].salience }
	}
	sort.Slice(pm.loadedPlugins, bySalienceDesc(pm.loadedPlugins))
	for hookName := range pm.hooks {
		sort.Slice(pm.hooks[hookName], bySalienceDesc(pm.hooks[hookName]))
	}

	return nil
}

// runByHook runs every plugin attached to sig, in salience order, threading
// the (possibly modified) entry from one plugin to the next. If no plugin
// implements sig, pm.hooks[sig] is simply empty and entry passes through
// unchanged — that's expected for hooks like OnInit/OnTick until a plugin
// file actually defines them, not an error.
func (pm *PluginManger) runByHook(sig string, entry DBEntry) (DBEntry, error) {
	if !pm.isRegistered(sig) {
		return entry, fmt.Errorf("hook %q is not registered", sig)
	}

	for _, plugin := range pm.hooks[sig] {
		var err error
		entry, err = pm.callHook(sig, entry, plugin)
		if err != nil {
			return entry, err
		}
	}
	return entry, nil
}

// callHook borrows this plugin's isolated LState from its own pool,
// marshals entry into a Lua table, calls the hook function (named sig) with
// it, and reads back the (possibly modified) table into entry. Unlike the
// old bucket-gated callBeginWithReturn, every field is writable by every
// hook — there's no Go-side type to gate on anymore, so this trusts plugin
// authors to only touch fields that make sense for their hook.
func (pm *PluginManger) callHook(sig string, entry DBEntry, plugin Plugin) (DBEntry, error) {
	L := plugin.pool.Get().(*lua.LState)
	defer plugin.pool.Put(L)

	t := L.NewTable()
	L.SetField(t, "Filename", lua.LString(entry.Filename))
	L.SetField(t, "Act", lua.LString(entry.Act))
	L.SetField(t, "Hash", lua.LString(entry.Hash))
	L.SetField(t, "Path", lua.LString(entry.Path))
	L.SetField(t, "Meta", lua.LString(entry.Meta))
	if entry.Barcode.Valid {
		L.SetField(t, "Barcode", lua.LString(entry.Barcode.String))
	} else {
		L.SetField(t, "Barcode", lua.LNil)
	}

	hookFunc := L.GetGlobal(sig)
	err := L.CallByParam(lua.P{
		Fn:      hookFunc,
		NRet:    1,
		Protect: true,
	}, t)
	if err != nil {
		return entry, err
	}

	result, ok := L.Get(-1).(*lua.LTable)
	L.Pop(1)
	if !ok {
		return entry, fmt.Errorf("%s() did not return a table", sig)
	}

	entry.Filename = result.RawGetString("Filename").String()
	entry.Act = result.RawGetString("Act").String()
	entry.Hash = result.RawGetString("Hash").String()
	entry.Path = result.RawGetString("Path").String()
	entry.Meta = result.RawGetString("Meta").String()
	if bc := result.RawGetString("Barcode"); bc.Type() == lua.LTNil {
		entry.Barcode = sql.NullString{}
	} else {
		entry.Barcode = sql.NullString{String: bc.String(), Valid: true}
	}

	return entry, nil
}

// registerHook declares that a hook name exists. Map assignment is
// idempotent — registering the same sig twice is a harmless no-op, unlike
// the old slice-append which would have just duplicated the entry.
func (pm *PluginManger) registerHook(sig string) error {
	if !pm.isRegistered(sig) {
		pm.registeredHooks[sig] = struct{}{}
	}
	return nil
}

// isRegistered reports whether sig has already been declared via
// registerHook — a single map lookup instead of scanning a slice.
func (pm *PluginManger) isRegistered(sig string) bool {
	_, exists := pm.registeredHooks[sig]
	return exists
}

// close is a no-op — sync.Pool has no drain method; states are released by the GC.
func (pm *PluginManger) close() {}

// listPlugins logs a one-line summary of how many plugins were loaded and
// how many hooks currently have at least one plugin attached.
func (pm *PluginManger) listPlugins() {
	Ok(fmt.Sprintf("%d plugins loaded across %d hooks", len(pm.loadedPlugins), len(pm.hooks)))
}

// reloadPlugins clears the current plugin/hook state and reloads everything
// from disk via loadPlugins, so a plugin file added, edited, or removed
// since the last load takes effect without restarting the process.
func (pm *PluginManger) reloadPlugins() error {
	Debug("Reloading plugins")
	pm.loadedPlugins = nil
	pm.hooks = make(map[string][]Plugin)

	pm.loadPlugins()
	return nil
}

// havePluginsChanged hashes every file in the plugin directory and compares
// it against the hashes recorded on the previous call, reporting true if any
// plugin was added, edited, or removed since then — including deletions,
// which show up as a tracked hash with no matching file left in dir.
func (pm *PluginManger) havePluginsChanged() (bool, error) {
	Debug("Checking if plugins have changed")

	entries, err := os.ReadDir(pm.dir)
	if err != nil {
		return false, err
	}

	changed := false
	seen := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue // There should be no subdirectories
		}
		seen[entry.Name()] = struct{}{}

		newHash, err := hashFile(filepath.Join(pm.dir, entry.Name()))
		if err != nil {
			continue
		}
		if oldHash, ok := pm.pluginHashes[entry.Name()]; ok && oldHash == newHash {
			continue
		}
		pm.pluginHashes[entry.Name()] = newHash
		changed = true
	}

	// A hash we're tracking for a file that's no longer on disk means that
	// file was removed — also a change, not just an addition/modification.
	for name := range pm.pluginHashes {
		if _, ok := seen[name]; !ok {
			delete(pm.pluginHashes, name)
			changed = true
		}
	}

	return changed, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
