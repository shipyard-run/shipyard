package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestExecLocalCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, execLocalRelative)
	defer cleanup()

	ex, err := c.FindResource("exec_local.setup_vault")
	assert.NoError(t, err)

	assert.Equal(t, "setup_vault", ex.Info().Name)
	assert.Equal(t, config.TypeExecLocal, ex.Info().Type)
	assert.Equal(t, config.PendingCreation, ex.Info().Status)
	assert.Equal(t, "./", ex.(*config.ExecLocal).WorkingDirectory)
	assert.True(t, ex.(*config.ExecLocal).Daemon)
}

func TestExecLocalSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, execLocalDisabled)
	defer cleanup()

	ex, err := c.FindResource("exec_local.setup_vault")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, ex.Info().Status)
}

var execLocalRelative = `
exec_local "setup_vault" {
  cmd = "./scripts/setup_vault.sh"
  args = [ "root", "abc" ] 
  working_directory = "./"
  daemon = true
}
`
var execLocalDisabled = `
exec_local "setup_vault" {
	disabled = true

  cmd = "./scripts/setup_vault.sh"
  args = [ "root", "abc" ] 
  working_directory = "./"
  daemon = true
}
`
