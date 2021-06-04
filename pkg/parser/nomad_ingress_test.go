package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNomadIngressCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, nomadIngressDefault)
	defer cleanup()

	cl, err := c.FindResource("nomad_ingress.test")
	assert.NoError(t, err)

	assert.Equal(t, "test", cl.Info().Name)
	assert.Equal(t, config.TypeNomadIngress, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}

func TestNomadIngressSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, nomadIngressDisabled)
	defer cleanup()

	cl, err := c.FindResource("nomad_ingress.test")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const nomadIngressDefault = `
network "test" {
	subnet = "10.0.0.0/24"
}

nomad_ingress "test" {
	cluster = "nomad_cluster.dc1"
	job = "a"
	group = "b"
	task = "c"
}
`

const nomadIngressDisabled = `
network "test" {
	subnet = "10.0.0.0/24"
}

nomad_ingress "test" {
	disabled = true
	cluster = "nomad_cluster.dc1"
	
	job = "a"
	group = "b"
	task = "c"
}
`
