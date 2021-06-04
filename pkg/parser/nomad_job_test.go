package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNomadJobCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, nomadJobDefault)
	defer cleanup()

	cl, err := c.FindResource("nomad_job.test")
	assert.NoError(t, err)

	assert.Equal(t, "test", cl.Info().Name)
	assert.Equal(t, config.TypeNomadJob, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}

func TestNomadJobSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, nomadJobDisabled)
	defer cleanup()

	cl, err := c.FindResource("nomad_job.test")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const nomadJobDefault = `
network "test" {
	subnet = "10.0.0.0/24"
}

nomad_job "test" {
  cluster = "nomad_cluster.dev"

  paths = ["./app_config/example2.nomad"]
  health_check {
    timeout = "60s"
    nomad_jobs = ["example_2"]
  }
}
`

const nomadJobDisabled = `
network "test" {
	subnet = "10.0.0.0/24"
}

nomad_job "test" {
	disabled = true
  cluster = "nomad_cluster.dev"

  paths = ["./app_config/example2.nomad"]
  health_check {
    timeout = "60s"
    nomad_jobs = ["example_2"]
  }
}
`
