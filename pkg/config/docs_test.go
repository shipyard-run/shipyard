package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesDocs(t *testing.T) {
	c := NewDocs("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeDocs, c.Type)
}
