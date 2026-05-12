package main

import (
	lua "github.com/yuin/gopher-lua"
)

func testPlugin() {
	L := lua.NewState()
	defer L.Close()
	if err := L.DoFile("./plugins/helloworld.lua"); err != nil {
		panic(err)
	}
}
