package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesK8sConfig(t *testing.T) {
	c := NewK8sConfig("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeK8sConfig, c.Type)
}
