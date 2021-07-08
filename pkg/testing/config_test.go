package testing

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfigCreatedWithCorrectValues(t *testing.T) {
	c := DefaultConfig()

	require.Equal(t, "info", c.LogLevel)
	require.True(t, c.CreateResources)
	require.True(t, c.DestroyResources)
	require.Equal(t, "./", c.FeaturesPath)
}

func TestConfigFromEnvCreatesConfigWithTags(t *testing.T) {
	os.Setenv("SY_TAG", "@foo,@bar")
	os.Args = append(os.Args, "--tags=@one,@two")

	c := ConfigFromEnv()

	t.Cleanup(func() {
		os.Unsetenv("SY_TAG")
	})

	require.Equal(t, []string{"@foo", "@bar", "@one", "@two"}, c.Tags)
}

func TestConfigFromEnvCreatesConfigWithVariables(t *testing.T) {
	os.Args = append(os.Args, `--var="foo=bar"`, `--var="count=1"`)

	c := ConfigFromEnv()

	require.Equal(t, map[string]string{"foo": "bar", "count": "1"}, c.Variables)
}
