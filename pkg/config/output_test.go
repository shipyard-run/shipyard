package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesOutput(t *testing.T) {
	c := NewOutput("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeOutput, c.Type)
}
