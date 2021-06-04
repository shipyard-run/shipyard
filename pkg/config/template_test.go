package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesTemplate(t *testing.T) {
	c := NewTemplate("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeTemplate, c.Type)
}
