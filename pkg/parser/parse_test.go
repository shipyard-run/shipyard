package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/shipyard-run/shipyard/pkg/clients/mocks"
	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/utils"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
)

func setup(t *testing.T) (*Parser, *mocks.Getter) {
	dir := t.TempDir()
	home := utils.HomeFolder()
	os.Setenv(utils.HomeEnvName(), dir)

	t.Cleanup(func() {
		os.Setenv(utils.HomeEnvName(), home)
	})

	g := &mocks.Getter{}
	g.On("Get", mock.Anything, mock.Anything).Return(nil)

	p := New(g)

	return p, g
}

func TestRunParsesBlueprintInMarkdownFormat(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/container")
	assert.NoError(t, err)

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	assert.NotNil(t, c.Blueprint)

	assert.Equal(t, "Nic Jackson", c.Blueprint.Author)
	assert.Equal(t, "Single Container Example", c.Blueprint.Title)
	assert.Equal(t, "container", c.Blueprint.Slug)
	assert.Equal(t, []string{"http://consul-http.ingress.shipyard.run:8500"}, c.Blueprint.BrowserWindows)
	assert.Equal(t, "SOMETHING", c.Blueprint.Environment[0].Key)
	assert.Equal(t, "else", c.Blueprint.Environment[0].Value)
	assert.Contains(t, c.Blueprint.Intro, "# Single Container")
	assert.Contains(t, c.Blueprint.HealthCheckTimeout, "30s")
}

func TestRunParsesBlueprintInHCLFormat(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/single_k3s_cluster")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	assert.NotNil(t, c.Blueprint)
}

func TestLoadsVariablesFiles(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/container")
	assert.NoError(t, err)

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	validEnv := false
	con := r.(*config.Container)
	for _, e := range con.Environment {
		// should contain a key called "something" with a value "else"
		if e.Key == "something" && e.Value == "blah blah" {
			validEnv = true
		}
	}

	assert.True(t, validEnv)
}

func TestLoadsVariablesFromOptionalFile(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/container")
	assert.NoError(t, err)

	absoluteVarsPath, err := filepath.Abs("../../examples/override.vars")
	assert.NoError(t, err)

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, absoluteVarsPath)
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	validEnv := false
	con := r.(*config.Container)
	for _, e := range con.Environment {
		// should contain a key called "something" with a value "else"
		if e.Key == "something" && e.Value == "else" {
			validEnv = true
		}
	}

	assert.True(t, validEnv)
}

func TestLoadsVariablesFilesForSingleFile(t *testing.T) {
	p, _ := setup(t)

	absoluteFilePath, err := filepath.Abs("../../examples/container/container.hcl")
	assert.NoError(t, err)

	absoluteVarsPath, err := filepath.Abs("../../examples/override.vars")
	assert.NoError(t, err)

	c := config.New()
	err = p.ParseFile(absoluteFilePath, c, map[string]string{}, absoluteVarsPath)
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	validEnv := false
	con := r.(*config.Container)
	for _, e := range con.Environment {
		// should contain a key called "something" with a value "else"
		if e.Key == "something" && e.Value == "else" {
			validEnv = true
		}
	}

	assert.True(t, validEnv)
}

func TestOverridesVariablesFilesWithFlag(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/container")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, map[string]string{"something": "else"}, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	validEnv := false
	con := r.(*config.Container)
	for _, e := range con.Environment {
		// should contain a key called "something" with a value "else"
		if e.Key == "something" && e.Value == "else" {
			validEnv = true
		}
	}

	assert.True(t, validEnv)
}

func TestOverridesVariablesFilesWithEnv(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/container")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("SY_VAR_something", "env")
	t.Cleanup(func() {
		os.Unsetenv("SY_VAR_something")
	})

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	validEnv := false
	con := r.(*config.Container)
	for _, e := range con.Environment {
		// should contain a key called "something" with a value "else"
		if e.Key == "something" && e.Value == "env" {
			validEnv = true
		}
	}

	assert.True(t, validEnv)
}

func TestVariablesSetFromDefault(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/variables/simple/")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	con := r.(*config.Container)

	assert.Equal(t, "onprem", con.Networks[0].Name)
}

func TestOverridesVariableDefaultsWithEnv(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/variables/simple/")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("SY_VAR_network", "cloud")
	t.Cleanup(func() {
		os.Unsetenv("SY_VAR_network")
	})

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	con := r.(*config.Container)
	assert.Equal(t, "cloud", con.Networks[0].Name)
}

func TestVariablesSetFromDefaultModule(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/variables/with_module/")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	con := r.(*config.Container)

	assert.Equal(t, "modulenetwork", con.Networks[0].Name)
}

func TestOverridesVariablesSetFromDefaultModuleWithEnv(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/variables/with_module/")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("SY_VAR_mod_network", "cloud")
	t.Cleanup(func() {
		os.Unsetenv("SY_VAR_mod_network")
	})

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	con := r.(*config.Container)
	assert.Equal(t, "cloud", con.Networks[0].Name)
}

func TestDoesNotLoadsVariablesFilesFromInsideModules(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/modules")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	validEnv := false
	con := r.(*config.Container)
	for _, e := range con.Environment {
		fmt.Println(e.Value)
		// should contain a key called "something" with a value "else"
		if e.Key == "something" && e.Value == "this is a module" {
			validEnv = true
		}
	}

	assert.True(t, validEnv)
}

func TestParseModuleCreatesResources(t *testing.T) {
	p, mg := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/modules")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	assert.Len(t, c.Resources, 11)

	// check depends on is set
	r, err := c.FindResource("docs.docs")
	assert.NoError(t, err)
	assert.Contains(t, r.Info().DependsOn, "container_ingress.consul-container-http-2")
	assert.Equal(t, r.Info().Module, "docs")

	// check the module is set on resources loaded as a module
	r, err = c.FindResource("container.consul")
	assert.NoError(t, err)
	assert.Equal(t, "consul", r.Info().Module)

	// Calls the getter for the remote module
	mg.AssertCalled(t, "Get", mock.Anything, mock.Anything)
}

func TestParseFileFunctionReadCorrectly(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/container")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	validEnv := false
	con := r.(*config.Container)
	for _, e := range con.Environment {
		// should contain a key called "something" with a value "else"
		if e.Key == "file" && e.Value == "this is the contents of a file" {
			validEnv = true
		}
	}

	assert.True(t, validEnv)
}

func TestParseAddsCacheDependencyToK8sResources(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/single_k3s_cluster")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()

	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	err = p.ParseReferences(c)
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("k8s_cluster.k3s")
	assert.NoError(t, err)

	assert.Contains(t, r.Info().DependsOn, fmt.Sprintf("%s.%s", string(config.TypeImageCache), utils.CacheResourceName))
}

func TestParseAddsCacheDependencyToNomadResources(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/nomad")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()

	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	err = p.ParseReferences(c)
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("nomad_cluster.dev")
	assert.NoError(t, err)

	assert.Contains(t, r.Info().DependsOn, fmt.Sprintf("%s.%s", string(config.TypeImageCache), utils.CacheResourceName))
}

func TestParseProcessesDisabled(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/container")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	// count the resources, should create 10
	assert.Len(t, c.Resources, 7)

	// check depends on is set
	r, err := c.FindResource("container.consul_disabled")
	assert.NoError(t, err)
	assert.Equal(t, r.Info().Disabled, true)
	assert.Equal(t, config.Disabled, r.Info().Status)
}

func TestParseProcessesDisabledOnModuleSettingChildDisabled(t *testing.T) {
	p, _ := setup(t)

	absoluteFolderPath, err := filepath.Abs("../../examples/modules")
	if err != nil {
		t.Fatal(err)
	}

	c := config.New()
	err = p.ParseFolder(absoluteFolderPath, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	assert.Len(t, c.Resources, 11)

	// check depends on is set
	r, err := c.FindResource("container.consul_disabled")
	assert.NoError(t, err)
	assert.Equal(t, true, r.Info().Disabled)

	//
	r, err = c.FindResource("exec_local.run")
	assert.NoError(t, err)
	assert.Equal(t, true, r.Info().Disabled)
}

func TestParseProcessesShipyardFunctions(t *testing.T) {
	p, _ := setup(t)

	tDir := t.TempDir()
	home := os.Getenv(utils.HomeEnvName())
	user := os.Getenv("USER")

	os.Setenv(utils.HomeEnvName(), tDir)
	os.Setenv("USER", "Nic")

	t.Cleanup(func() {
		os.Setenv(utils.HomeEnvName(), home)
		os.Setenv("USER", user)
	})

	absoluteFolderPath, err := filepath.Abs("../../examples/functions")
	assert.NoError(t, err)

	absoluteFilePath, err := filepath.Abs("../../examples/functions/container.hcl")
	assert.NoError(t, err)

	absoluteVarsPath, err := filepath.Abs("../../examples/override.vars")
	assert.NoError(t, err)

	_, kubeConfigFile, kubeConfigDockerFile := utils.CreateKubeConfigPath("dc1")

	ip, _ := utils.GetLocalIPAndHostname()
	clusterConf, _ := utils.GetClusterConfig("nomad_cluster.dc1")
	clusterIP := clusterConf.APIAddress(utils.LocalContext)

	c := config.New()
	err = p.ParseFile(absoluteFilePath, c, map[string]string{}, absoluteVarsPath)
	assert.NoError(t, err)

	// check variable has been interpolated
	r, err := c.FindResource("container.consul")
	assert.NoError(t, err)

	cc := r.(*config.Container)

	assert.Equal(t, absoluteFolderPath, cc.EnvVar["file_dir"])
	assert.Equal(t, absoluteFilePath, cc.EnvVar["file_path"])
	assert.Equal(t, os.Getenv("USER"), cc.EnvVar["env"])
	assert.Equal(t, kubeConfigFile, cc.EnvVar["k8s_config"])
	assert.Equal(t, kubeConfigDockerFile, cc.EnvVar["k8s_config_docker"])
	assert.Equal(t, utils.HomeFolder(), cc.EnvVar["home"])
	assert.Equal(t, utils.ShipyardHome(), cc.EnvVar["shipyard"])
	assert.Contains(t, cc.EnvVar["file"], "version=\"consul:1.8.1\"")
	assert.Equal(t, utils.GetDataFolder("mine"), cc.EnvVar["data"])
	assert.Equal(t, utils.GetDockerIP(), cc.EnvVar["docker_ip"])
	assert.Equal(t, utils.GetDockerHost(), cc.EnvVar["docker_host"])
	assert.Equal(t, ip, cc.EnvVar["shipyard_ip"])
	assert.Equal(t, clusterIP, cc.EnvVar["cluster_api"])
}
