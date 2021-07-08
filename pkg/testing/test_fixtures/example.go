package main

import (
	"github.com/shipyard-run/shipyard/pkg/testing"
)

func main() {
	c := testing.DefaultConfig()
	r := testing.NewRunner(c)

	// setup the default steps to stop test failures
	r.RegisterStep(`I expect a step to be called`, func() error { return nil })

	r.Run()
}
