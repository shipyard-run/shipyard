package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestK8sClusterCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, clusterDefault)
	defer cleanup()

	cl, err := c.FindResource("k8s_cluster.testing")
	assert.NoError(t, err)

	assert.Equal(t, "testing", cl.Info().Name)
	assert.Equal(t, config.TypeK8sCluster, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}

func TestK8sClusterSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, clusterDisabled)
	defer cleanup()

	cl, err := c.FindResource("k8s_cluster.testing")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const clusterDefault = `
k8s_cluster "testing" {
	network {
		name = "network.test"
	}
	driver = "k3s"
}
`
const clusterDisabled = `
k8s_cluster "testing" {
	disabled = true

	network {
		name = "network.test"
	}
	driver = "k3s"
}
`
