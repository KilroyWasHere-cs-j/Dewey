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
	// pools is keyed by plugin filename. Each plugin has its own pool of LStates
	// loaded only with that plugin's file — this ensures Begin() resolves to the
	// correct plugin's function rather than whichever plugin happened to load last.
	pools     map[string]*sync.Pool
	FilterMap map[string]Plugin
	ScriptMap map[string]Plugin
	InitMap   map[string]Plugin
	TickMap   map[string]Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		pools:     make(map[string]*sync.Pool),
		FilterMap: make(map[string]Plugin),
		ScriptMap: make(map[string]Plugin),
		InitMap:   make(map[string]Plugin),
		TickMap:   make(map[string]Plugin),
	}
}

// Close is a no-op — sync.Pool has no drain method; states are released by the GC.
func (pm *PluginManager) Close() {}

func (pm *PluginManager) LoadPlugins() error {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		Warn("Unable to read plugin directory: " + err.Error())
		return err
	}

	// Temporary state used only for plugin-type discovery; discarded after this call.
	// Plugins are loaded one at a time so WhoAmI identifies each file individually.
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
		L.Pop(2) // clean WhoAmI return values off the stack

		sVal, ok := salienceLV.(lua.LNumber)
		if !ok {
			Warn(fmt.Sprintf("plugin %s: salience must be a number, got %s", entry.Name(), salienceLV.Type()))
			sVal = 0 // Default to 0 if not a number
			continue
		}
		salience := int(sVal)

		// Build a per-plugin state pool. Capturing pluginFile by value in the
		// closure avoids the loop variable capture bug.
		pluginFile := filepath.Join(pluginDir, entry.Name())
		pm.pools[entry.Name()] = &sync.Pool{
			New: func() any {
				state := lua.NewState()
				if err := state.DoFile(pluginFile); err != nil {
					Warn("pool: failed to load " + pluginFile + ": " + err.Error())
				}
				return state
			},
		}

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
				entry, err = pm.callBeginWithReturn(entry, plugin)
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
				entry, err = pm.callBeginWithReturn(entry, plugin)
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

func (pm *PluginManager) callBeginWithReturn(entry DBEntry, plugin Plugin) (DBEntry, error) {
	// Borrow this plugin's isolated LState from its own pool. Using per-plugin
	// pools ensures Begin() is the function defined by this plugin, not one
	// overwritten by a later-loaded plugin in a shared state.
	pool, ok := pm.pools[plugin.name]
	if !ok {
		return entry, fmt.Errorf("no state pool found for plugin: %s", plugin.name)
	}
	L := pool.Get().(*lua.LState)
	defer pool.Put(L)

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

	if plugin.pluigntype == Filter {
		entry.Path = result.RawGetString("Path").String()
	}

	if plugin.pluigntype == Script {
		entry.Meta = result.RawGetString("Meta").String()
		entry.Act = result.RawGetString("Act").String()
	}
	return entry, nil
}

func (pm *PluginManager) ListPlugins() {
	Ok(fmt.Sprintf("%d filter, %d script, %d init, %d tick plugins loaded",
		len(pm.FilterMap), len(pm.ScriptMap), len(pm.InitMap), len(pm.TickMap)))
}
