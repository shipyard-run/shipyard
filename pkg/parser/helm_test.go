package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestHelmCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, helmDefault)
	defer cleanup()

	h, err := c.FindResource("helm.testing")
	assert.NoError(t, err)

	assert.Equal(t, "testing", h.Info().Name)
	assert.Equal(t, config.TypeHelm, h.Info().Type)
	assert.Equal(t, config.PendingCreation, h.Info().Status)
}

func TestHelmSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, helmDisabled)
	defer cleanup()

	h, err := c.FindResource("helm.testing")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, h.Info().Status)
}

const helmDefault = `
helm "testing" {
	cluster = "cluster.k3s"

	chart = "test"
	values = "test"
}
`

const helmDisabled = `
helm "testing" {
	disabled = true

	cluster = "cluster.k3s"

	chart = "test"
	values = "test"
}
`
