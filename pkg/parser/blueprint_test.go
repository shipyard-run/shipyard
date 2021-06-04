package parser

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func setupBlueprints(t *testing.T, contents string) (*config.Config, func()) {
	p, _ := setup(t)

	dir, cleanup := createTestFiles(t)
	createNamedFile(t, dir, "*.yard", contents)

	c := &config.Config{}
	err := p.ParseFolder(dir, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	return c, cleanup
}

func TestBlueprintCreatesCorrectly(t *testing.T) {
	c, cleanup := setupBlueprints(t, blueprintDefault)
	defer cleanup()

	// should have created a blueprint
	bp := c.Blueprint
	assert.NotNil(t, bp)

	assert.Equal(t, "default blueprint", bp.Title)
	assert.Equal(t, "Keyser Söze", bp.Author)
	assert.Contains(t, bp.Slug, "This is")
	assert.Len(t, bp.BrowserWindows, 2)
	assert.Equal(t, "http://www.google.com", bp.BrowserWindows[0])
	assert.Len(t, bp.Environment, 2)
	assert.Equal(t, "DEBUG", bp.Environment[1].Key)
	assert.Equal(t, "true", bp.Environment[1].Value)
}

func TestBlueprintValidationInvalidBrowser(t *testing.T) {
	c, cleanup := setupBlueprints(t, blueprintInvalidBrowser)
	defer cleanup()

	errs := c.Blueprint.Validate()
	assert.Len(t, errs, 1)
}

var blueprintDefault = `
title = "default blueprint"
author = "Keyser Söze"
slug = <<EOF
	This is the slug contents
EOF

browser_windows = [
	"http://www.google.com",
	"https://www.something.com",
]

env {
	key = "KUBECONFIG"
	value = "/root/.kube/.something.yml"
}

env {
	key = "DEBUG"
	value = "true"
}
`

var blueprintInvalidBrowser = `
browser_windows = [
	"",
	"https://www.something.com",
]
`
