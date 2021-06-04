package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesIngress(t *testing.T) {
	c := NewIngress("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeIngress, c.Type)
}
