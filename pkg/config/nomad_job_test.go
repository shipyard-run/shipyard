package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesNomadJob(t *testing.T) {
	c := NewNomadJob("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeNomadJob, c.Type)
}
