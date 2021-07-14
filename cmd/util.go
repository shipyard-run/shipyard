package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/TwinProduction/go-color"
	"github.com/docker/docker/api/types"
	"github.com/hashicorp/go-hclog"
	v1 "k8s.io/api/core/v1"
)

func createLogger() hclog.Logger {

	opts := &hclog.LoggerOptions{Color: hclog.AutoColor}

	// set the log level
	if lev := os.Getenv("LOG_LEVEL"); lev != "" {
		opts.Level = hclog.LevelFromString(lev)
	}

	return hclog.New(opts)
}

const (
	// parsed from stack
	typeBpTypes = iota
	typeBpContainers
	typeBpClusters
	// parsed after connection
	typeConnContainers
	typeConnPods
)

// stackI is an interface that can be used for shell auto-complete
// based on either static stack file or after client connections
type stackI interface {
	// blueprint from stack
	printStack(out *os.File)
	// for auto-completion
	toNames(typ int, key string) []string
	// docker, kubernetes connection
	addConn(key string, list interface{})
}

type stacks struct {
	stack map[string][]string    // type<->container/clusterName
	conn  map[string]interface{} // type<->[]types.Container/[]v1.pod
}

// newStack returns a stackI interface that is initialised with the provided
// stack blueprint
func newStack(m map[string][]string) stackI {
	return &stacks{stack: m, conn: make(map[string]interface{})}
}

// printStack prints stack
func (stack *stacks) printStack(out *os.File) {
	for typ, names := range stack.stack {
		_, _ = fmt.Fprintf(out, "%-8s\t%s\n", typ, names)
	}
}

// addConn adds a new value to the connections.
func (stack *stacks) addConn(key string, list interface{}) {
	stack.conn[key] = list
}

// toNames returns the value for the key type casted to []string
func (stack *stacks) toNames(typ int, key string) []string {
	t := new([]string)
	switch typ {
	case typeBpContainers:
		return stack.stack[key] // key == containers
	case typeBpClusters:
		return stack.stack[key] // key == k8s-cluster
	case typeBpTypes:
		for typ, _ := range stack.stack {
			*t = append(*t, typ)
		}
	case typeConnContainers:
		for _, c := range stack.conn[key].([]types.Container) {
			*t = append(*t, c.Names[0][1:])
		}
	case typeConnPods:
		for _, c := range stack.conn[key].([]v1.Pod) {
			*t = append(*t, c.Name)
		}
	}
	return *t
}

// colorI is an interface that returns a color from a list of colors
type colorI interface {
	nextColor() string
}
type colour struct {
	colors         []string
	nextColorIndex int
	mx             sync.Mutex
}

// nextColor returns a color from a list of colors
func (c *colour) nextColor() string {
	c.mx.Lock()
	defer c.mx.Unlock()
	if c.nextColorIndex != len(c.colors) {
		c.nextColorIndex++
		return c.colors[c.nextColorIndex]
	}
	c.nextColorIndex = 0
	return c.colors[c.nextColorIndex]
}

// newColor returns a colorI interface
func newColor() colorI {
	c := colour{}
	c.colors = append(c.colors, color.Blue)
	c.colors = append(c.colors, color.Green)
	c.colors = append(c.colors, color.Purple)
	c.colors = append(c.colors, color.Yellow)
	c.colors = append(c.colors, color.Bold)
	c.colors = append(c.colors, color.Gray)
	c.colors = append(c.colors, color.Red)

	c.nextColorIndex = 0
	c.mx = sync.Mutex{}
	return &c
}
