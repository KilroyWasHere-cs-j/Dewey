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

func (pm *PluginManager) LoadPlugins() {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		Fatal("Unable to read plugin directory: " + err.Error())
	}

	for _, entry := range entries {
		Debug(entry.Name())

		if err := pm.L.DoFile(filepath.Join(pluginDir, entry.Name())); err != nil {
			panic(err)
		}

		// 2. Retrieve the function
		whoAmIFunc := pm.L.GetGlobal("WhoAmI")

		// 3. Call the function
		err = pm.L.CallByParam(lua.P{
			Fn:      whoAmIFunc,
			NRet:    2, // Number of return values
			Protect: true,
		}) // Arguments

		if err != nil {
			Fatal(err.Error())
		}

		// 4. Retrieve return value, using magic numbers
		pluginType := pm.L.Get(-2).String()
		salienceLV := pm.L.Get(-1)

		sVal, ok := salienceLV.(lua.LNumber)
		if !ok {
			Warn(fmt.Sprintf("plugin %s: salience must be a number, got %s", entry.Name(), salienceLV.Type()))
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
}

func (pm *PluginManager) RunPlugins(targetBucket string) {
	switch targetBucket {
	case "filter":
		Debug("Running filter plugins...")
		for _, plugin := range pm.FilterMap {
			Debug(fmt.Sprintf("Running filter plugin(s) %s", plugin.name))
			if err := pm.L.DoFile(filepath.Join(pluginDir, plugin.name)); err != nil {
				Warn(err.Error())
			}

			Debug("Calling Begin function")
			// 2. Retrieve the function
			beginFunc := pm.L.GetGlobal("Begin")

			// 3. Call the function
			err := pm.L.CallByParam(lua.P{
				Fn:      beginFunc,
				NRet:    0, // Number of return values
				Protect: true,
			}) // Arguments

			if err != nil {
				Fatal(err.Error())
			}
			Debug("No errors")
		}

		Debug("Calling Begin function")
		// 2. Retrieve the function
		beginFunc := pm.L.GetGlobal("Begin")

		// 3. Call the function
		err := pm.L.CallByParam(lua.P{
			Fn:      beginFunc,
			NRet:    0, // Number of return values
			Protect: true,
		}) // Arguments

		if err != nil {
			Fatal(err.Error())
		}
		Debug("No errors")

	case "script":
		Debug("Running script plugins...")
		for _, plugin := range pm.ScriptMap {
			Debug(fmt.Sprintf("Running script plugin(s) %s", plugin.name))
			if err := pm.L.DoFile(filepath.Join(pluginDir, plugin.name)); err != nil {
				Warn(err.Error())
			}

			Debug("Calling Begin function")
			// 2. Retrieve the function
			beginFunc := pm.L.GetGlobal("Begin")

			// 3. Call the function
			err := pm.L.CallByParam(lua.P{
				Fn:      beginFunc,
				NRet:    0, // Number of return values
				Protect: true,
			}) // Arguments

			if err != nil {
				Fatal(err.Error())
			}
			Debug("No errors")
		}
	case "init":
		Debug("Running init plugins...")
		for _, plugin := range pm.InitMap {
			Debug(fmt.Sprintf("Running init plugin(s) %s", plugin.name))
			if err := pm.L.DoFile(filepath.Join(pluginDir, plugin.name)); err != nil {
				Warn(err.Error())
			}

			Debug("Calling Begin function")
			// 2. Retrieve the function
			beginFunc := pm.L.GetGlobal("Begin")

			// 3. Call the function
			err := pm.L.CallByParam(lua.P{
				Fn:      beginFunc,
				NRet:    0, // Number of return values
				Protect: true,
			}) // Arguments

			if err != nil {
				Fatal(err.Error())
			}
			Debug("No errors")
		}

	// TODO: This goes nowhere
	case "tick":
		Debug("Running tick plugins...")
		for _, plugin := range pm.TickMap {
			Debug(fmt.Sprintf("Running tick plugin(s) %s", plugin.name))
			if err := pm.L.DoFile(filepath.Join(pluginDir, plugin.name)); err != nil {
				Warn(err.Error())
			}

			Debug("Calling Begin function")
			// 2. Retrieve the function
			beginFunc := pm.L.GetGlobal("Begin")

			// 3. Call the function
			err := pm.L.CallByParam(lua.P{
				Fn:      beginFunc,
				NRet:    0, // Number of return values
				Protect: true,
			}) // Arguments

			if err != nil {
				Fatal(err.Error())
			}
			Debug("No errors")
		}
	}
}

func (pm *PluginManager) ListPlugins() {
	Debug("")
	Debug("====================")
	Debug(fmt.Sprintf("%d filter plugins loaded", len(pm.FilterMap)))
	Debug(fmt.Sprintf("%d script plugins loaded", len(pm.ScriptMap)))
	Debug(fmt.Sprintf("%d init plugins loaded", len(pm.InitMap)))
	Debug("====================")
	Debug("")
}
