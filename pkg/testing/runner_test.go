package testing

import (
	"testing"

	"github.com/shipyard-run/shipyard/pkg/shipyard/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupRunner(t *testing.T) (Runner, *mocks.Engine) {
	c := DefaultConfig()
	c.FeaturesPath = "./test_fixtures"

	me := &mocks.Engine{}
	me.On("ApplyWithVariables", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	me.On("Destroy", mock.Anything, mock.Anything).Return(nil)

	r := NewRunner(c, me)

	// setup the default steps to stop test failures
	r.RegisterStep(`I expect a step to be called`, func() error { return nil })

	return r, me
}

func TestRunCallsCustomSteps(t *testing.T) {
	r, _ := setupRunner(t)

	stepCalled := false
	r.RegisterStep(`I expect a step to be called`, func() error {
		stepCalled = true
		return nil
	})

	err := r.Run()
	require.NoError(t, err)

	require.True(t, stepCalled)
}

func TestRunCallsBeforeScenario(t *testing.T) {
	r, _ := setupRunner(t)

	beforeCalled := false
	r.BeforeScenario(func() error {
		beforeCalled = true
		return nil
	})

	err := r.Run()
	require.NoError(t, err)

	require.True(t, beforeCalled)
}

func TestRunCallsAfterScenario(t *testing.T) {
	r, _ := setupRunner(t)

	afterCalled := false
	r.AfterScenario(func() error {
		afterCalled = true
		return nil
	})

	err := r.Run()
	require.NoError(t, err)

	require.True(t, afterCalled)
}

func TestRunCallsBeforeSuite(t *testing.T) {
	r, _ := setupRunner(t)

	beforeCalled := false
	r.BeforeSuite(func() error {
		beforeCalled = true
		return nil
	})

	err := r.Run()
	require.NoError(t, err)

	require.True(t, beforeCalled)
}

func TestRunCallsAfterSuite(t *testing.T) {
	r, _ := setupRunner(t)

	afterCalled := false
	r.AfterSuite(func() error {
		afterCalled = true
		return nil
	})

	err := r.Run()
	require.NoError(t, err)

	require.True(t, afterCalled)
}
