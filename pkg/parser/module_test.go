package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModuleCreatesCorrectly(t *testing.T) {
	c, _, cleanup := setupTestConfig(t, moduleDefault)
	defer cleanup()

	_, err := c.FindResource("module.testing")
	assert.Error(t, err) // should not add a resource for the module
}

const moduleDefault = `
module "testing" {
	source = "../../examples/single_file"
}
`
