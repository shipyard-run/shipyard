package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesContainer(t *testing.T) {
	c := NewContainer("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeContainer, c.Type)
}
