package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesNomadCluster(t *testing.T) {
	c := NewNomadCluster("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeNomadCluster, c.Type)
}
