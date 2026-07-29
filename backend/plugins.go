package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// LoadPlugins runs, since LoadPlugins only checks plugin files against
	// already-registered names.
	registeredHooks map[string]struct{}
	// hooks maps a registered hook name to every plugin that implements it
	// (i.e. defines a global function with that exact name), sorted by
	// salience descending. This is the actual dispatch table RunByHook uses.
	hooks         map[string][]Plugin
	loadedPlugins []Plugin
}

func NewPluginManger() *PluginManger {
	return &PluginManger{
		registeredHooks: make(map[string]struct{}),
		hooks:           make(map[string][]Plugin),
		loadedPlugins:   make([]Plugin, 0),
	}
}

// Plugin loader
func (pm *PluginManger) LoadPlugins() error {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		Warn("Unable to read plugin directory: " + err.Error())
		return err
	}

	for _, entry := range entries {
		// Fresh state per plugin file for discovery, closed at the end of
		// this iteration (issue #216) — a single state reused across every
		// DoFile call let Lua globals (WhoAmI, etc.) persist between files,
		// so a plugin missing WhoAmI silently inherited the previous
		// plugin's type/salience instead of failing to classify.
		L := lua.NewState()
		if err := L.DoFile(filepath.Join(pluginDir, entry.Name())); err != nil {
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
		pluginFile := filepath.Join(pluginDir, entry.Name())
		pool := &sync.Pool{
			New: func() any {
				state := lua.NewState()
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

// RunByHook runs every plugin attached to sig, in salience order, threading
// the (possibly modified) entry from one plugin to the next. If no plugin
// implements sig, pm.hooks[sig] is simply empty and entry passes through
// unchanged — that's expected for hooks like OnInit/OnTick until a plugin
// file actually defines them, not an error.
func (pm *PluginManger) RunByHook(sig string, entry DBEntry) (DBEntry, error) {
	if !pm.IsRegistered(sig) {
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
	L.SetField(t, "Barcode", lua.LString(entry.Barcode))

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
	entry.Barcode = result.RawGetString("Barcode").String()

	return entry, nil
}

// RegisterHook declares that a hook name exists. Map assignment is
// idempotent — registering the same sig twice is a harmless no-op, unlike
// the old slice-append which would have just duplicated the entry.
func (pm *PluginManger) RegisterHook(sig string) error {
	if !pm.IsRegistered(sig) {
		pm.registeredHooks[sig] = struct{}{}
	}
	return nil
}

// IsRegistered reports whether sig has already been declared via
// RegisterHook — a single map lookup instead of scanning a slice.
func (pm *PluginManger) IsRegistered(sig string) bool {
	_, exists := pm.registeredHooks[sig]
	return exists
}

// Close is a no-op — sync.Pool has no drain method; states are released by the GC.
func (pm *PluginManger) Close() {}

// ListPlugins logs a one-line summary of how many plugins were loaded and
// how many hooks currently have at least one plugin attached.
func (pm *PluginManger) ListPlugins() {
	Ok(fmt.Sprintf("%d plugins loaded across %d hooks", len(pm.loadedPlugins), len(pm.hooks)))
}
