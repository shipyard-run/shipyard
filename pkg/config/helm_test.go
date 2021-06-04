package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesHelm(t *testing.T) {
	c := NewHelm("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeHelm, c.Type)
}
