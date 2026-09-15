package main

import (
	"os"
)


func main() {
	err := SpinUp()
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}
}
