package testing

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunCallsEngineApply(t *testing.T) {
	r, me := setupRunner(t)

	err := r.Run()
	require.NoError(t, err)

	me.AssertCalled(t, "ApplyWithVariables", mock.Anything, mock.Anything, mock.Anything)
}

func TestRunCallsEngineDestroy(t *testing.T) {
	r, me := setupRunner(t)

	err := r.Run()
	require.NoError(t, err)

	me.AssertCalled(t, "Destroy", "", true)
}
