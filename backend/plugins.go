package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

type PluginType string

const (
	Filter PluginType = "filter"
	Init   PluginType = "init"
	Script PluginType = "script"
	Tick   PluginType = "tick"
)

type Plugin struct {
	salience   int
	name       string
	pluigntype PluginType
}

type PluginManager struct {
	// pool holds pre-loaded LStates. Each goroutine borrows one, uses it,
	// then returns it — keeping Lua VM state isolated between concurrent calls.
	pool      sync.Pool
	FilterMap map[string]Plugin
	ScriptMap map[string]Plugin
	InitMap   map[string]Plugin
	TickMap   map[string]Plugin
}

func NewPluginManager() *PluginManager {
	pm := &PluginManager{
		FilterMap: make(map[string]Plugin),
		ScriptMap: make(map[string]Plugin),
		InitMap:   make(map[string]Plugin),
		TickMap:   make(map[string]Plugin),
	}

	// New is called lazily when the pool has no free state available.
	// Each state is fully loaded with all plugin files so it is ready to use.
	pm.pool = sync.Pool{
		New: func() any {
			L := lua.NewState()
			entries, err := os.ReadDir(pluginDir)
			if err != nil {
				return L
			}
			for _, entry := range entries {
				if err := L.DoFile(filepath.Join(pluginDir, entry.Name())); err != nil {
					Warn("pool: failed to load plugin " + entry.Name() + ": " + err.Error())
				}
			}
			return L
		},
	}

	return pm
}

// Close is a no-op under the pool model — sync.Pool does not expose a drain
// method and pooled states are released by the GC on process exit.
func (pm *PluginManager) Close() {}

func (pm *PluginManager) LoadPlugins() error {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		Warn("Unable to read plugin directory: " + err.Error())
		return err
	}

	// Temporary state used only for plugin-type discovery; discarded after this call.
	// Plugins are loaded one at a time so that WhoAmI identifies each file individually.
	L := lua.NewState()
	defer L.Close()

	for _, entry := range entries {
		Debug(entry.Name())

		if err := L.DoFile(filepath.Join(pluginDir, entry.Name())); err != nil {
			Warn("Unable to load plugin " + entry.Name() + ": " + err.Error())
			continue
		}

		// Identify the type and salience of the plugin
		whoAmIFunc := L.GetGlobal("WhoAmI")
		err = L.CallByParam(lua.P{
			Fn:      whoAmIFunc,
			NRet:    2, // Number of return values
			Protect: true,
		})
		if err != nil {
			Warn("Unable to call WhoAmI: " + err.Error())
		}

		// Yes these are magic numbers don't touch them
		pluginType := L.Get(-2).String()
		salienceLV := L.Get(-1)

		sVal, ok := salienceLV.(lua.LNumber)
		if !ok {
			Warn(fmt.Sprintf("plugin %s: salience must be a number, got %s", entry.Name(), salienceLV.Type()))
			sVal = 0 // Default to 0 if not a number
			continue
		}
		salience := int(sVal)

		switch pluginType {
		case "filter":
			pm.FilterMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience, pluigntype: Filter}
		case "script":
			pm.ScriptMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience, pluigntype: Script}
		case "init":
			pm.InitMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience, pluigntype: Init}
		case "tick":
			pm.TickMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience, pluigntype: Tick}
		}
	}
	return nil
}

func (pm *PluginManager) RunPlugins(targetBucket PluginType) func(DBEntry) (DBEntry, error) {
	switch targetBucket {
	case Filter:
		return func(entry DBEntry) (DBEntry, error) {
			for _, plugin := range pm.FilterMap {
				Debug(fmt.Sprintf("Running filter plugin: %s", plugin.name))
				var err error
				entry, err = pm.callBeginWithReturn(entry, plugin.pluigntype)
				if err != nil {
					return entry, err
				}
			}
			return entry, nil
		}

	case Script:
		return func(entry DBEntry) (DBEntry, error) {
			for _, plugin := range pm.ScriptMap {
				Debug(fmt.Sprintf("Running script plugin: %s", plugin.name))
				var err error
				entry, err = pm.callBeginWithReturn(entry, plugin.pluigntype)
				if err != nil {
					return entry, err
				}
			}
			return entry, nil
		}

	case Init:
		return func(entry DBEntry) (DBEntry, error) {
			// one-time setup logic
			return entry, nil
		}

	case Tick:
		return func(entry DBEntry) (DBEntry, error) {
			// periodic logic
			return entry, nil
		}

	default:
		return func(entry DBEntry) (DBEntry, error) {
			return entry, fmt.Errorf("unknown plugin bucket: %s", targetBucket)
		}
	}
}

func (pm *PluginManager) callBeginWithReturn(entry DBEntry, pluginType PluginType) (DBEntry, error) {
	// Borrow an isolated LState from the pool; return it when done so the
	// next caller can reuse it without racing.
	L := pm.pool.Get().(*lua.LState)
	defer pm.pool.Put(L)

	t := L.NewTable()
	L.SetField(t, "Filename", lua.LString(entry.Filename))
	L.SetField(t, "Act", lua.LString(entry.Act))
	L.SetField(t, "Hash", lua.LString(entry.Hash))
	L.SetField(t, "Path", lua.LString(entry.Path))
	L.SetField(t, "Meta", lua.LString(entry.Meta))
	L.SetField(t, "Barcode", lua.LString(entry.Barcode))

	beginFunc := L.GetGlobal("Begin")
	err := L.CallByParam(lua.P{
		Fn:      beginFunc,
		NRet:    1,
		Protect: true,
	}, t)
	if err != nil {
		return entry, err
	}

	// Read back the (possibly modified) table
	result, ok := L.Get(-1).(*lua.LTable)
	L.Pop(1)
	if !ok {
		return entry, fmt.Errorf("Begin() did not return a table")
	}

	if pluginType == Filter {
		entry.Path = result.RawGetString("Path").String()
	}

	if pluginType == Script {
		entry.Meta = result.RawGetString("Meta").String()
		entry.Act = result.RawGetString("Act").String()
	}
	return entry, nil
}

func (pm *PluginManager) ListPlugins() {
	Ok(fmt.Sprintf("%d filter, %d script, %d init, %d tick plugins loaded",
		len(pm.FilterMap), len(pm.ScriptMap), len(pm.InitMap), len(pm.TickMap)))
}
