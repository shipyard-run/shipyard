package parser

import (
	"path/filepath"
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestExecRemoteCreatesCorrectly(t *testing.T) {
	c, dir, cleanup := setupTestConfig(t, execRemoteRelative)
	defer cleanup()

	ex, err := c.FindResource("exec_remote.setup_vault")
	assert.NoError(t, err)

	assert.Equal(t, "setup_vault", ex.Info().Name)
	assert.Equal(t, config.TypeExecRemote, ex.Info().Type)
	assert.Equal(t, config.PendingCreation, ex.Info().Status)

	assert.Equal(t, "hashicorp/vault:latest", ex.(*config.ExecRemote).Image.Name)

	assert.Len(t, ex.(*config.ExecRemote).Volumes, 1)
	assert.Equal(t, filepath.Join(dir, "/scripts"), ex.(*config.ExecRemote).Volumes[0].Source)
}

func TestExecRemoteSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, execRemoteDisabled)
	defer cleanup()

	ex, err := c.FindResource("exec_remote.setup_vault")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, ex.Info().Status)
}

var execRemoteRelative = `
network "cloud" {
	subnet = "192.158.32.12"
}

exec_remote "setup_vault" {
  image {
	  name = "hashicorp/vault:latest"
  }
  network {
	  name = "network.cloud"
	}

	cmd = "/scripts/setup_vault.sh"

  volume {
	  source = "./scripts"
	  destination = "/files"
  }
}
`
var execRemoteDisabled = `
network "cloud" {
	subnet = "192.158.32.12"
}

exec_remote "setup_vault" {
	disabled = true

  image {
	  name = "hashicorp/vault:latest"
  }
  network {
	  name = "network.cloud"
	}

	cmd = "/scripts/setup_vault.sh"

  volume {
	  source = "./scripts"
	  destination = "/files"
  }
}
`
