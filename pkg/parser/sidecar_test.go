package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestSidecarCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, sidecarDefault)
	defer cleanup()

	cl, err := c.FindResource("sidecar.test")
	assert.NoError(t, err)

	assert.Equal(t, "test", cl.Info().Name)
	assert.Equal(t, config.TypeSidecar, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}

func TestSidecarSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, sidecarDisabled)
	defer cleanup()

	cl, err := c.FindResource("sidecar.test")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const sidecarDefault = `
sidecar "test" {
	target = "container.test"
	image {
		name = "consul"
	}
}
`

const sidecarDisabled = `
sidecar "test" {
	disabled = true
	target = "container.test"
	image {
		name = "consul"
	}
}
`
