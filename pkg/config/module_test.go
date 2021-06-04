package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesModule(t *testing.T) {
	c := NewModule("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeModule, c.Type)
}
