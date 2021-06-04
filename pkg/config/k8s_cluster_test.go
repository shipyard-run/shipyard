package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreatesK8sCluster(t *testing.T) {
	c := NewK8sCluster("abc")

	assert.Equal(t, "abc", c.Name)
	assert.Equal(t, TypeK8sCluster, c.Type)
}
