package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/evo3cx/reenvoy"
)

func init() {
	// Seed the default rand Source with current time to produce better random
	// numbers used with splay
	rand.Seed(time.Now().UnixNano())
}

func main() {
	cli := reenvoy.NewCLI(os.Stdout, os.Stderr)
	os.Exit(cli.Run(os.Args))
}
