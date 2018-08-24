package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/evo3cx/reenvoy"
)

func init() {
	// Seed the default rand Source with current time to produce better random
	// numbers used with splay
	rand.Seed(time.Now().UnixNano())
}

func main() {
	options := reenvoy.SpawnOptions{
		Command: "echo",
		Args:    []string{"hello", "world"},
	}

	reenvoy.Start(options)

	fmt.Scanln()
}
