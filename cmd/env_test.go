package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/shipyard-run/shipyard/pkg/utils"
	"github.com/spf13/cobra"
	assert "github.com/stretchr/testify/require"
)

func setupEnvState(t *testing.T, state string) *cobra.Command {
	// set the home folder to a tmpFolder for the tests
	dir := t.TempDir()

	home := os.Getenv(utils.HomeEnvName())
	os.Setenv(utils.HomeEnvName(), dir)

	// write the state file
	if state != "" {
		os.MkdirAll(utils.StateDir(), os.ModePerm)
		f, err := os.Create(utils.StatePath())
		assert.NoError(t, err)

		defer f.Close()
		_, err = f.WriteString(state)
		assert.NoError(t, err)
	}

	t.Cleanup(func() {
		os.Setenv(utils.HomeEnvName(), home)
	})

	return newEnvCmd(nil)
}

func TestSetsEnvironmentVariables(t *testing.T) {
	en := setupEnvState(t, envState)
	out := bytes.NewBufferString("")
	en.SetOutput(out)

	err := en.Execute()
	assert.NoError(t, err)

	assert.Contains(t, `export foo="bar"`, out.String())
	assert.Contains(t, `export abc="12\"3"`, out.String())
	assert.Contains(t, `export apples="pears"`, out.String())
}

func TestUnsetsEnvironmentVariables(t *testing.T) {
	en := setupEnvState(t, envState)
	en.Flags().Set("unset", "true")
	out := bytes.NewBufferString("")
	en.SetOutput(out)

	err := en.Execute()
	assert.NoError(t, err)

	assert.Contains(t, `unset foo`, out.String())
	assert.Contains(t, `unset abc`, out.String())
	assert.Contains(t, `unset apples`, out.String())
}

var envState = `
{
  "blueprint": {
    "environment": {
      "apples":"pears",
      "abc":"12\"3"
    }
  },
  "resources": [
	{
      "name": "foo",
      "status": "applied",
      "value": "bar",
      "type": "output"
	}
  ]
}
`

