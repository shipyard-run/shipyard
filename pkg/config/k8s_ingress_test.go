package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesK8sIngress(t *testing.T) {
	c := NewK8sIngress("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeK8sIngress, c.Type)
}
