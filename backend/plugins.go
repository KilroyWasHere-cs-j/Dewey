package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// pool is this plugin's own LState pool, loaded only with that plugin's
	// file — this ensures Begin() resolves to the correct plugin's function
	// rather than whichever plugin happened to load last.
	pool *sync.Pool
}

type PluginManager struct {
	// Each bucket is sorted by salience (highest first) at the end of
	// LoadPlugins, so RunEntryPlugins/RunInit/RunTick execute plugins in salience order instead
	// of the random order map iteration would give.
	FilterMap []Plugin
	ScriptMap []Plugin
	InitMap   []Plugin
	TickMap   []Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		FilterMap: make([]Plugin, 0),
		ScriptMap: make([]Plugin, 0),
		InitMap:   make([]Plugin, 0),
		TickMap:   make([]Plugin, 0),
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
		pool := &sync.Pool{
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
			pm.FilterMap = append(pm.FilterMap, Plugin{name: entry.Name(), salience: salience, pluigntype: Filter, pool: pool})
		case "script":
			pm.ScriptMap = append(pm.ScriptMap, Plugin{name: entry.Name(), salience: salience, pluigntype: Script, pool: pool})
		case "init":
			pm.InitMap = append(pm.InitMap, Plugin{name: entry.Name(), salience: salience, pluigntype: Init, pool: pool})
		case "tick":
			pm.TickMap = append(pm.TickMap, Plugin{name: entry.Name(), salience: salience, pluigntype: Tick, pool: pool})
		}
	}

	// Highest salience runs first within each bucket.
	bySalienceDesc := func(plugins []Plugin) func(i, j int) bool {
		return func(i, j int) bool { return plugins[i].salience > plugins[j].salience }
	}
	sort.Slice(pm.FilterMap, bySalienceDesc(pm.FilterMap))
	sort.Slice(pm.ScriptMap, bySalienceDesc(pm.ScriptMap))
	sort.Slice(pm.InitMap, bySalienceDesc(pm.InitMap))
	sort.Slice(pm.TickMap, bySalienceDesc(pm.TickMap))

	return nil
}

// RunEntryPlugins returns a closure that runs every plugin in the given
// bucket against a DBEntry, threading the (possibly modified) entry through
// each plugin in salience order. Only Filter and Script plugins operate on
// a DBEntry — Init/Tick are argless, side-effecting only, and run via
// RunInit/RunTick instead.
func (pm *PluginManager) RunEntryPlugins(targetBucket PluginType) func(DBEntry) (DBEntry, error) {
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

	default:
		return func(entry DBEntry) (DBEntry, error) {
			return entry, fmt.Errorf("unknown plugin bucket: %s", targetBucket)
		}
	}
}

// runArglessPlugins calls Begin() with no arguments on every plugin in the
// given bucket, in salience order. Used for Init/Tick plugins, which have
// nothing to do with DBEntry — Init runs once at startup, Tick once per
// daemon cycle (see startDaemon in utils.go). One plugin failing is logged
// and doesn't stop the rest of the bucket from running.
func (pm *PluginManager) runArglessPlugins(bucketName string, plugins []Plugin) {
	for _, plugin := range plugins {
		Debug(fmt.Sprintf("Running %s plugin: %s", bucketName, plugin.name))

		L := plugin.pool.Get().(*lua.LState)
		beginFunc := L.GetGlobal("Begin")
		err := L.CallByParam(lua.P{
			Fn:      beginFunc,
			NRet:    0,
			Protect: true,
		})
		plugin.pool.Put(L)

		if err != nil {
			Warn(fmt.Sprintf("%s plugin %s failed: %s", bucketName, plugin.name, err.Error()))
		}
	}
}

// RunInit calls Begin() on every init plugin. Meant to run once at startup,
// before any files are processed.
func (pm *PluginManager) RunInit() {
	pm.runArglessPlugins("init", pm.InitMap)
}

// RunTick calls Begin() on every tick plugin. Meant to run once per daemon
// cycle, alongside cache cleanup and the backup pass.
func (pm *PluginManager) RunTick() {
	pm.runArglessPlugins("tick", pm.TickMap)
}

func (pm *PluginManager) callBeginWithReturn(entry DBEntry, plugin Plugin) (DBEntry, error) {
	// Borrow this plugin's isolated LState from its own pool. Using per-plugin
	// pools ensures Begin() is the function defined by this plugin, not one
	// overwritten by a later-loaded plugin in a shared state.
	L := plugin.pool.Get().(*lua.LState)
	defer plugin.pool.Put(L)

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
