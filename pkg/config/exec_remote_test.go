package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesExecRemote(t *testing.T) {
	c := NewExecRemote("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeExecRemote, c.Type)
}
