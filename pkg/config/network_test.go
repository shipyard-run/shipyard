package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesNetwork(t *testing.T) {
	c := NewNetwork("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeNetwork, c.Type)
}
