package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNomadClusterCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, nomadClusterDefault)
	defer cleanup()

	cl, err := c.FindResource("nomad_cluster.test")
	assert.NoError(t, err)

	assert.Equal(t, "test", cl.Info().Name)
	assert.Equal(t, config.TypeNomadCluster, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}

func TestNomadClusterSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, nomadClusterDisabled)
	defer cleanup()

	cl, err := c.FindResource("nomad_cluster.test")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const nomadClusterDefault = `
network "test" {
	subnet = "10.0.0.0/24"
}

nomad_cluster "test" {
}
`

const nomadClusterDisabled = `
network "test" {
	subnet = "10.0.0.0/24"
}

nomad_cluster "test" {
	disabled = true
}
`
