package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesExecLocal(t *testing.T) {
	c := NewExecLocal("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeExecLocal, c.Type)
}
