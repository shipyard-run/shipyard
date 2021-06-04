package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestIngressCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, ingressDefault)
	defer cleanup()

	cl, err := c.FindResource("ingress.testing")
	assert.NoError(t, err)

	assert.Equal(t, "testing", cl.Info().Name)
	assert.Equal(t, config.TypeIngress, cl.Info().Type)
	assert.Equal(t, config.PendingCreation, cl.Info().Status)
}
func TestIngressSetsDisabled(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, ingressDisabled)
	defer cleanup()

	cl, err := c.FindResource("ingress.testing")
	assert.NoError(t, err)

	assert.Equal(t, config.Disabled, cl.Info().Status)
}

const ingressDefault = `
network "test" {
	subnet = "10.0.0.0/24"
}

k8s_cluster "testing" {
	network {
		name = "network.test"
	}
	driver = "k3s"
}

ingress "testing" {
	destination {
		driver = "k8s"
		config {
			port = 8080
		}
	}
	
	source {
		driver = "k8s"
		config {
			port = 8080
		}
	}
}
`
const ingressDisabled = `
network "test" {
	subnet = "10.0.0.0/24"
}

k8s_cluster "testing" {

	network {
		name = "network.test"
	}
	driver = "k3s"
}

ingress "testing" {
	disabled = true

	destination {
		driver = "k8s"
		config {
			port = 8080
		}
	}
	
	source {
		driver = "k8s"
		config {
			port = 8080
		}
	}
}
`
