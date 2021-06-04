package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNetworkCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, networkDefault)
	defer cleanup()

	cl, err := c.FindResource("network.test")
	assert.NoError(t, err)

	assert.Equal(t, "test", cl.Info().Name)
	assert.Equal(t, config.TypeNetwork, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}

func TestNetworkSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, networkDisabled)
	defer cleanup()

	cl, err := c.FindResource("network.test")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const networkDefault = `
network "test" {
	subnet = "10.0.0.0/24"
}
`

const networkDisabled = `
network "test" {
	disabled = true
	subnet = "10.0.0.0/24"
}
`
