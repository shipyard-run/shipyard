package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestK8sIngressCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, k8sIngressDefault)
	defer cleanup()

	cl, err := c.FindResource("k8s_ingress.testing")
	assert.NoError(t, err)

	assert.Equal(t, "testing", cl.Info().Name)
	assert.Equal(t, config.TypeK8sIngress, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}
func TestK8sIngressSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, k8sIngressDisabled)
	defer cleanup()

	cl, err := c.FindResource("k8s_ingress.testing")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const k8sIngressDefault = `
network "test" {
	subnet = "10.0.0.0/24"
}

k8s_cluster "testing" {
	network {
		name = "network.test"
	}
	driver = "k3s"
}

k8s_ingress "testing" {
	cluster = "k8s_cluster.testing"
}
`
const k8sIngressDisabled = `
network "test" {
	subnet = "10.0.0.0/24"
}

k8s_cluster "testing" {
	network {
		name = "network.test"
	}
	driver = "k3s"
}

k8s_ingress "testing" {
	disabled = true
	cluster = "k8s_cluster.testing"
}
`
