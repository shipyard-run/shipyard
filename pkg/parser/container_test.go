package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestContainerCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, containerDefault)
	defer cleanup()

	co, err := c.FindResource("container.testing")
	assert.NoError(t, err)

	assert.Equal(t, "testing", co.Info().Name)
	assert.Equal(t, config.TypeContainer, co.Info().Type)
	assert.Equal(t, config.PendingCreation, co.Info().Status)
}

func TestContainerSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, containerDisabled)
	defer cleanup()

	co, err := c.FindResource("container.testing")
	assert.NoError(t, err)

	assert.Equal(t, "testing", co.Info().Name)
	assert.Equal(t, config.Disabled, co.Info().Status)
}

const containerDefault = `
network "test" {
	subnet = "10.0.0.0/24"
}

container "testing" {
	network {
		name = "network.test"
	}
	image {
		name = "consul"
	}
}
`

const containerDisabled = `
network "test" {
	subnet = "10.0.0.0/24"
}

container "testing" {
	disabled = true

	network {
		name = "network.test"
	}
	image {
		name = "consul"
	}
}
`
