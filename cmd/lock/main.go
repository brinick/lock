package main

import (
	"fmt"
	"os"
)

func main() {
	app := createApp()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprint(os.Stderr, fmt.Sprintf("%v\n", err))
		os.Exit(1)
	}
}
