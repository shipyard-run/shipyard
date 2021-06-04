package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesContainerIngress(t *testing.T) {
	c := NewContainerIngress("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeContainerIngress, c.Type)
}
