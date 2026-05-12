package main

import (
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

type PluginType int

const (
	Filter PluginType = iota // 0
	Script                   // 1
)

type Plugin struct {
	Type     PluginType
	salience int
}

var L *lua.LState
var PluginMap = map[string]Plugin{}

func loadPlugins() {
	L := lua.NewState()
	defer L.Close()

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		Fatal("Unable to read plugin directory: " + err.Error())
	}

	for _, entry := range entries {
		Debug(entry.Name())

		if err := L.DoFile(filepath.Join(pluginDir, entry.Name())); err != nil {
			panic(err)
		}

		// 2. Retrieve the function
		whoAmIFunc := L.GetGlobal("WhoAmI")

		// 3. Call the function
		err = L.CallByParam(lua.P{
			Fn:      whoAmIFunc,
			NRet:    2, // Number of return values
			Protect: true,
		}) // Arguments

		if err != nil {
			Fatal(err.Error())
		}

		// 4. Retrieve return value
		pluginType := L.Get(-2)
		salience := L.Get(-1)
		PluginMap[entry.Name()] = Plugin{Type: PluginType(pluginType.Type()), salience: int(salience.Type())}
	}

}

func testPlugin() {

}

func callFilter() {
	// Code for calling a plugin of the filter type
}

func callScript() {
	// Code for calling a plugin of the script type
}

// // 1. Create argument table
// args := L.NewTable()
// L.RawSet(args, lua.LNumber(1), lua.LString("wack"))
// // L.RawSet(args, lua.LNumber(2), lua.LString("arg2"))

// // 2. Set global "arg"
// L.SetGlobal("arg", args)

// if err := L.DoFile(filepath.Join(pluginDir, entries[0].Name())); err != nil {
// 	Warn("Unable to load-run plugin: " + err.Error())
// }
// // Inside your Lua file (data.lua)
// return {
//     name = "Gopher",
//     age = 10
// }

// // Inside your Go code
// L := lua.NewState()
// defer L.Close()
// if err := L.DoFile("data.lua"); err != nil {
//     panic(err)
// }
// // Get the table returned by the file
// result := L.Get(-1)
// if tbl, ok := result.(*lua.LTable); ok {
//     fmt.Println(tbl.RawGetString("name"))
// }

// import (
//     "github.com/yuin/gopher-lua"
//     "log"
// )

// func main() {
//     L := lua.NewState()
//     defer L.Close()

//     // 1. Load the script
//     if err := L.DoString(`
//         function sayHello(name)
//             return "Hello, " .. name
//         end
//     `); err != nil {
//         log.Fatal(err)
//     }

//     // 2. Retrieve the function
//     helloFunc := L.GetGlobal("sayHello")

//     // 3. Call the function
//     err := L.CallByParam(lua.P{
//         Fn:      helloFunc,
//         NRet:    1, // Number of return values
//         Protect: true,
//     }, lua.LString("World")) // Arguments

//     if err != nil {
//         log.Fatal(err)
//     }

//     // 4. Retrieve return value
//     ret := L.Get(-1)
//     L.Pop(1)
//     println(ret.String()) // Output: Hello, World
// }
