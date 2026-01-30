package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 && args[0] == "local" {
		fmt.Println("local mode: not yet implemented")
		os.Exit(0)
	}

	fmt.Println("ccc: not yet implemented")
}
