package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestK8sConfigCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, k8sConfigValid)
	defer cleanup()

	cc, err := c.FindResource("k8s_config.test")
	assert.NoError(t, err)

	assert.Equal(t, "test", cc.Info().Name)
	assert.Equal(t, config.TypeK8sConfig, cc.Info().Type)
	assert.Equal(t, config.PendingCreation, cc.Info().Status)

	assert.Equal(t, "/tmp/files", cc.(*config.K8sConfig).Paths[0])
	assert.True(t, cc.(*config.K8sConfig).WaitUntilReady)
}

func TestK8sConfigSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, k8sConfigDisabled)
	defer cleanup()

	cc, err := c.FindResource("k8s_config.test")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cc.Info().Status)
}

func TestMakesPathAbsolute(t *testing.T) {
	c, base, cleanup := setupTestConfig(t, k8sConfigValid)
	defer cleanup()

	kc, err := c.FindResource("k8s_config.test")
	assert.NoError(t, err)

	assert.Contains(t, kc.(*config.K8sConfig).Paths[1], base)
}

var k8sConfigValid = `
k8s_cluster "cloud" {
  driver  = "k3s" // default
  version = "1.16.0"

  nodes = 1 // default

  network {
	  name = "network.k8s"
  }
}

k8s_config "test" {
	cluster = "cluster.cloud"
	paths = ["/tmp/files","./myfiles"]
	wait_until_ready = true

	health_check {
		timeout = "30s"
		http = "http://www.google.com"
	}
}
`
var k8sConfigDisabled = `
k8s_cluster "cloud" {
  driver  = "k3s" // default
  version = "1.16.0"

  nodes = 1 // default

  network {
	  name = "network.k8s"
  }
}

k8s_config "test" {
	disabled = true

	cluster = "cluster.cloud"
	paths = ["/tmp/files","./myfiles"]
	wait_until_ready = true

	health_check {
		timeout = "30s"
		http = "http://www.google.com"
	}
}
`
