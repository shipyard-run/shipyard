package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesSidecar(t *testing.T) {
	c := NewSidecar("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeSidecar, c.Type)
}
