package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestOutputCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, outputDefault)
	defer cleanup()

	cl, err := c.FindResource("output.test")
	assert.NoError(t, err)

	assert.Equal(t, "test", cl.Info().Name)
	assert.Equal(t, config.TypeOutput, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}

func TestOutputSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, outputDisabled)
	defer cleanup()

	cl, err := c.FindResource("output.test")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const outputDefault = `
output "test" {
	value = "abcc"
}
`

const outputDisabled = `
output "test" {
	disabled = true
	value = "abcc"
}
`
