package main

import (
	"fmt"
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

type Plugin struct {
	salience int
	name     string
}

type PluginManager struct {
	L         *lua.LState
	FilterMap map[string]Plugin
	ScriptMap map[string]Plugin
	InitMap   map[string]Plugin
	TickMap   map[string]Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		L:         lua.NewState(),
		FilterMap: make(map[string]Plugin),
		ScriptMap: make(map[string]Plugin),
		InitMap:   make(map[string]Plugin),
		TickMap:   make(map[string]Plugin),
	}
}

func (pm *PluginManager) Close() {
	// TODO: call End() on each plugin
	pm.L.Close()
}

func (pm *PluginManager) LoadPlugins() error {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		Warn("Unable to read plugin directory: " + err.Error())
		return err // Return the error as it is a fatal error
	}

	for _, entry := range entries {
		Debug(entry.Name())

		if err := pm.L.DoFile(filepath.Join(pluginDir, entry.Name())); err != nil {
			Warn("Unable to load plugin " + entry.Name() + ": " + err.Error())
			continue // Just skip this plugin and move on to the next one
		}

		// Identify the type and salience of the plugin
		whoAmIFunc := pm.L.GetGlobal("WhoAmI")
		err = pm.L.CallByParam(lua.P{
			Fn:      whoAmIFunc,
			NRet:    2, // Number of return values
			Protect: true,
		})
		if err != nil {
			Warn("Unable to call WhoAmI: " + err.Error())
		}

		// Yes these are magic numbers don't touch them
		pluginType := pm.L.Get(-2).String()
		salienceLV := pm.L.Get(-1)

		sVal, ok := salienceLV.(lua.LNumber)
		if !ok {
			Warn(fmt.Sprintf("plugin %s: salience must be a number, got %s", entry.Name(), salienceLV.Type()))
			sVal = 0 // Default to 0 if not a number
			continue
		}
		salience := int(sVal)

		switch pluginType {
		case "filter":
			pm.FilterMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience}
		case "script":
			pm.ScriptMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience}
		case "init":
			pm.InitMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience}
		case "tick":
			pm.TickMap[entry.Name()] = Plugin{name: entry.Name(), salience: salience}
		}
	}
	return nil
}

func (pm *PluginManager) RunPlugins(targetBucket string) func(DBEntry) (DBEntry, error) {
	switch targetBucket {
	case "filter":
		return func(entry DBEntry) (DBEntry, error) {
			for _, plugin := range pm.FilterMap {
				Debug(fmt.Sprintf("Running filter plugin: %s", plugin.name))
				if err := pm.L.DoFile(filepath.Join(pluginDir, plugin.name)); err != nil {
					return entry, err
				}
				var err error
				entry, err = pm.callBeginWithReturn(entry)
				if err != nil {
					return entry, err
				}
			}
			return entry, nil
		}

	case "script":
		return func(entry DBEntry) (DBEntry, error) {
			for _, plugin := range pm.ScriptMap {
				Debug(fmt.Sprintf("Running script plugin: %s", plugin.name))
				if err := pm.L.DoFile(filepath.Join(pluginDir, plugin.name)); err != nil {
					return entry, err
				}
				var err error
				entry, err = pm.callBeginWithReturn(entry)
				if err != nil {
					return entry, err
				}
			}
			return entry, nil
		}

	case "init":
		return func(entry DBEntry) (DBEntry, error) {
			// one-time setup logic
			return entry, nil
		}

	case "tick":
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

func (pm *PluginManager) callBeginWithReturn(entry DBEntry) (DBEntry, error) {
	t := pm.L.NewTable()
	pm.L.SetField(t, "Filename", lua.LString(entry.Filename))
	pm.L.SetField(t, "Act", lua.LString(entry.Act))
	pm.L.SetField(t, "Hash", lua.LString(entry.Hash))
	pm.L.SetField(t, "Path", lua.LString(entry.Path))
	pm.L.SetField(t, "Meta", lua.LString(entry.Meta))

	beginFunc := pm.L.GetGlobal("Begin")
	err := pm.L.CallByParam(lua.P{
		Fn:      beginFunc,
		NRet:    1,
		Protect: true,
	}, t)
	if err != nil {
		return entry, err
	}

	// Read back the (possibly modified) table
	result, ok := pm.L.Get(-1).(*lua.LTable)
	pm.L.Pop(1)
	if !ok {
		return entry, fmt.Errorf("Begin() did not return a table")
	}

	entry.Act = result.RawGetString("Act").String()
	entry.Meta = result.RawGetString("Meta").String()
	Debug(fmt.Sprintf("Plugin modified entry: Act=%s, Meta=%s", entry.Act, entry.Meta))
	return entry, nil
}

func (pm *PluginManager) ListPlugins() {
	Debug("")
	Debug("====================")
	Debug(fmt.Sprintf("%d filter plugins loaded", len(pm.FilterMap)))
	Debug(fmt.Sprintf("%d script plugins loaded", len(pm.ScriptMap)))
	Debug(fmt.Sprintf("%d init plugins loaded", len(pm.InitMap)))
	Debug(fmt.Sprintf("%d tick plugins loaded", len(pm.TickMap)))
	Debug("====================")
	Debug("")
}
