package shipyard

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/go-hclog"
	clmocks "github.com/shipyard-run/shipyard/pkg/clients/mocks"
	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/parser"
	"github.com/shipyard-run/shipyard/pkg/providers"
	"github.com/shipyard-run/shipyard/pkg/providers/mocks"
	"github.com/shipyard-run/shipyard/pkg/utils"

	assert "github.com/stretchr/testify/require"
)

var lock = sync.Mutex{}

func setupTests(returnVals map[string]error) (Engine, *[]*mocks.MockProvider, func()) {
	return setupTestsBase(returnVals, "")
}

func setupTestsWithState(returnVals map[string]error, state string) (Engine, *[]*mocks.MockProvider, func()) {
	return setupTestsBase(returnVals, state)
}

func setupState(state string) func() {
	// set the home folder to a tmpFolder for the tests
	dir, err := ioutils.TempDir("", "")
	if err != nil {
		panic(err)
	}

	home := os.Getenv(utils.HomeEnvName())
	os.Setenv(utils.HomeEnvName(), dir)

	// write the state file
	if state != "" {
		os.MkdirAll(utils.StateDir(), os.ModePerm)
		f, err := os.Create(utils.StatePath())
		if err != nil {
			panic(err)
		}
		defer f.Close()
		_, err = f.WriteString(state)
		if err != nil {
			panic(err)
		}
	}

	return func() {
		os.Setenv(utils.HomeEnvName(), home)
		os.RemoveAll(dir)
	}
}

func setupTestsBase(returnVals map[string]error, state string) (Engine, *[]*mocks.MockProvider, func()) {
	log.SetOutput(ioutil.Discard)

	p := &[]*mocks.MockProvider{}

	cl := &Clients{}
	cl.Getter = &clmocks.Getter{}
	cl.Parser = parser.New(cl.Getter)
	e := &EngineImpl{
		clients:     cl,
		log:         hclog.NewNullLogger(),
		getProvider: generateProviderMock(p, returnVals),
	}

	return e, p, setupState(state)
}

func generateProviderMock(mp *[]*mocks.MockProvider, returnVals map[string]error) getProviderFunc {
	return func(c config.Resource, cc *Clients) providers.Provider {
		lock.Lock()
		defer lock.Unlock()

		m := mocks.New(c)

		val := returnVals[c.Info().Name]
		m.On("Create").Return(val)
		m.On("Destroy").Return(val)

		*mp = append(*mp, m)
		return m
	}
}

func getTestFiles(tests string) string {
	e, err := os.Executable()
	if err != nil {
		panic(err)
	}
	path := path.Dir(e)
	return filepath.Join(path, "/examples", tests)
}

func TestNewCreatesClients(t *testing.T) {
	e, err := New(hclog.NewNullLogger())
	assert.NoError(t, err)

	cl := e.GetClients()

	assert.NotNil(t, cl.Kubernetes)
	assert.NotNil(t, cl.Helm)
	assert.NotNil(t, cl.Command)
	assert.NotNil(t, cl.HTTP)
	assert.NotNil(t, cl.Nomad)
	assert.NotNil(t, cl.Getter)
	assert.NotNil(t, cl.Browser)
	assert.NotNil(t, cl.ImageLog)
	assert.NotNil(t, cl.Connector)
}

func TestApplyWithSingleFile(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	_, err := e.Apply("../../examples/single_file/container.hcl")
	assert.NoError(t, err)

	assert.Equal(t, "onprem", (*mp)[0].Config().Info().Name)

	// can either be consul or the image cache
	assert.Contains(t, []string{"consul", "docker-cache"}, (*mp)[2].Config().Info().Name)
}

func TestApplyAddsImageCache(t *testing.T) {
	e, _, cleanup := setupTests(nil)
	defer cleanup()

	_, err := e.Apply("../../examples/single_file/container.hcl")
	assert.NoError(t, err)

	dc := e.ResourceCountForType(string(config.TypeImageCache))
	assert.Equal(t, 1, dc)
}

func TestApplyWithSingleFileAndVariables(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	_, err := e.ApplyWithVariables("../../examples/single_file/container.hcl", nil, "../../examples/single_file/default.vars")
	assert.NoError(t, err)

	assert.Equal(t, "onprem", (*mp)[0].Config().Info().Name)

	assert.Contains(t, []string{"consul", "docker-cache", "local_connector"}, (*mp)[1].Config().Info().Name)
}

func TestApplyCallsProviderInCorrectOrder(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	_, err := e.Apply("../../examples/single_k3s_cluster")
	assert.NoError(t, err)

	// should have called in order
	assert.Equal(t, "cloud", (*mp)[0].Config().Info().Name)

	// due to paralel nature of the DAG, these two elements will be first
	assert.Contains(t, []string{"docker-cache", "k3s", "local_connector"}, (*mp)[1].Config().Info().Name)

	// due to paralel nature of the DAG, these two elements can appear in any order
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault", "vault-http", "consul-lan", "k3s"}, (*mp)[2].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault", "vault-http", "consul-lan", "k3s"}, (*mp)[3].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault", "vault-http", "consul-lan", "k3s"}, (*mp)[4].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault", "vault-http", "consul-lan", "k3s"}, (*mp)[5].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault", "vault-http", "consul-lan", "k3s"}, (*mp)[6].Config().Info().Name)
}

func TestApplyCallsProviderCreateForEachProvider(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	_, err := e.Apply("../../examples/single_k3s_cluster")
	assert.NoError(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Create", 8)
	//assert.Len(t, res, 4)
}

func TestApplyNotCallsProviderDestroyAndCreateForResourcesDisabled(t *testing.T) {
	e, mp, cleanup := setupTestsWithState(nil, disabledState)
	defer cleanup()

	_, err := e.Apply("")
	assert.NoError(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 0)
	testAssertMethodCalled(t, mp, "Create", 2) // ImageCache is always created

	// check state of disabled remains disabled
	c := config.New()
	c.FromJSON(utils.StatePath())

	r, err := c.FindResource("container.dc1")
	assert.NoError(t, err)
	assert.Equal(t, config.Disabled, r.Info().Status)
}

func TestApplyCallsProviderDestroyAndCreateForResourcesFailed(t *testing.T) {
	e, mp, cleanup := setupTestsWithState(nil, failedState)
	defer cleanup()

	_, err := e.Apply("")
	assert.NoError(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 1)
	testAssertMethodCalled(t, mp, "Create", 2) // ImageCache is always created
}

func TestApplyCallsProviderDestroyForResourcesPendingModification(t *testing.T) {
	e, mp, cleanup := setupTestsWithState(nil, modificationState)
	defer cleanup()

	_, err := e.Apply("")
	assert.NoError(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 1)
	testAssertMethodCalled(t, mp, "Create", 2) // ImageCache is always created
}

func TestApplyReturnsErrorWhenProviderDestroyForResourcesPendingorFailed(t *testing.T) {
	e, mp, cleanup := setupTestsWithState(map[string]error{"dc1": fmt.Errorf("boom")}, failedState)
	defer cleanup()

	_, err := e.Apply("")
	assert.Error(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 1)
	testAssertMethodCalled(t, mp, "Create", 1) // ImageCache is always created
}

func TestApplyCallsProviderGenerateErrorStopsExecution(t *testing.T) {
	e, mp, cleanup := setupTests(map[string]error{"cloud": fmt.Errorf("boom")})
	defer cleanup()

	_, err := e.Apply("../../examples/single_k3s_cluster")
	assert.Error(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Create", 1)
}

func TestApplyCallsProviderCreateErrorStopsExecution(t *testing.T) {
	e, mp, cleanup := setupTests(map[string]error{"cloud": fmt.Errorf("boom")})
	defer cleanup()

	_, err := e.Apply("../../examples/single_k3s_cluster")
	assert.Error(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Create", 1)
}

func TestApplySetsStatusForEachResource(t *testing.T) {
	e, mp, cleanup := setupTestsWithState(nil, mergedState)
	defer cleanup()

	_, err := e.Apply("")
	assert.NoError(t, err)

	// should only call create and destroy for the cache as this is pending update
	testAssertMethodCalled(t, mp, "Create", 1) // ImageCache is always created
}

func TestParseConfig(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	err := e.ParseConfig("../../examples/single_file/container.hcl")
	assert.NoError(t, err)

	assert.Equal(t, 3, e.(*EngineImpl).config.ResourceCount())

	// should not have crated any providers
	testAssertMethodCalled(t, mp, "Create", 0)
	testAssertMethodCalled(t, mp, "Destroy", 0)
}

func TestParseWithVariables(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	err := e.ParseConfigWithVariables("../../examples/single_file/container.hcl", nil, "../../examples/single_file/default.vars")
	assert.NoError(t, err)

	assert.Equal(t, 3, e.(*EngineImpl).config.ResourceCount())

	// should not have crated any providers
	testAssertMethodCalled(t, mp, "Create", 0)
	testAssertMethodCalled(t, mp, "Destroy", 0)
}

func TestDestroyCallsProviderDestroyForEachProvider(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	err := e.Destroy("../../examples/single_k3s_cluster", true)
	assert.NoError(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 8)
}

func TestDestroyNotCallsProviderDestroyForResourcesDisabled(t *testing.T) {
	e, mp, cleanup := setupTestsWithState(nil, disabledState)
	defer cleanup()

	err := e.Destroy("", true)
	assert.NoError(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 2)
	testAssertMethodCalled(t, mp, "Create", 0) // ImageCache is always created

	// check state of disabled remains disabled
	c := config.New()
	c.FromJSON(utils.StatePath())

	_, err = c.FindResource("container.dc1")
	assert.Error(t, err) // resource should not exist
}

func TestDestroyCallsProviderGenerateErrorStopsExecution(t *testing.T) {
	e, mp, cleanup := setupTests(map[string]error{"k3s": fmt.Errorf("boom")})
	defer cleanup()

	err := e.Destroy("../../examples/single_k3s_cluster", true)
	assert.Error(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 6)
}

func TestDestroyFailSetsStatus(t *testing.T) {
	e, mp, cleanup := setupTests(map[string]error{"cloud": fmt.Errorf("boom")})
	defer cleanup()

	err := e.Destroy("../../examples/single_k3s_cluster", true)
	assert.Error(t, err)

	// should have call create for each provider
	testAssertMethodCalled(t, mp, "Destroy", 8)
	assert.Equal(t, config.Failed, (*mp)[7].Config().Info().Status)
}

func TestDestroyCallsProviderDestroyInCorrectOrder(t *testing.T) {
	e, mp, cleanup := setupTests(nil)
	defer cleanup()

	err := e.Destroy("../../examples/single_k3s_cluster", true)
	assert.NoError(t, err)

	// due to paralel nature of the DAG, these elements can appear in any order
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault-http", "vault", "connector", "consul-lan", "k3s"}, (*mp)[0].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault-http", "vault", "connector", "consul-lan", "k3s"}, (*mp)[1].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault-http", "vault", "connector", "consul-lan", "k3s"}, (*mp)[2].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault-http", "vault", "connector", "consul-lan", "k3s"}, (*mp)[3].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault-http", "vault", "connector", "consul-lan", "k3s"}, (*mp)[4].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault-http", "vault", "connector", "consul-lan", "k3s"}, (*mp)[5].Config().Info().Name)
	assert.Contains(t, []string{"docker-cache", "consul-http", "consul", "vault-http", "vault", "connector", "consul-lan", "k3s"}, (*mp)[6].Config().Info().Name)

	// network should be last to be removed
	assert.Equal(t, "cloud", (*mp)[7].Config().Info().Name)
}

func testAssertMethodCalled(t *testing.T, p *[]*mocks.MockProvider, method string, n int, args ...interface{}) {
	callCount := 0

	for _, pm := range *p {
		// cast the provider into a mock
		for _, c := range pm.Calls {
			if c.Method == method {
				callCount++
			}
		}
	}

	if callCount != n {
		t.Fatalf("Expected %d calls got %d", n, callCount)
	}
}

var failedState = `
{
  "blueprint": null,
  "resources": [
	{
      "name": "dc1",
      "status": "failed",
      "subnet": "10.15.0.0/16",
      "type": "network"
	}
  ]
}
`

var modificationState = `
{
  "blueprint": null,
  "resources": [
	{
      "name": "dc1",
      "status": "pending_modification",
      "subnet": "10.15.0.0/16",
      "type": "network"
	}
  ]
}
`

var mergedState = `
{
  "blueprint": null,
  "resources": [
	{
      "name": "dc1",
      "status": "pending_update",
      "subnet": "10.15.0.0/16",
      "type": "network"
	}
  ]
}
`

var disabledState = `
{
  "blueprint": null,
  "resources": [
	{
      "name": "dc1",
      "status": "pending_creation",
      "subnet": "10.15.0.0/16",
      "type": "network"
	},
	{
      "name": "dc1",
      "status": "disabled",
      "type": "container"
	}
  ]
}
`
