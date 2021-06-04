package shipyard

import (

	// "fmt"

	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform/dag"
	"github.com/hashicorp/terraform/tfdiags"
	"golang.org/x/xerrors"

	// "github.com/mitchellh/mapstructure"
	"github.com/shipyard-run/shipyard/pkg/clients"
	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/parser"
	"github.com/shipyard-run/shipyard/pkg/providers"
	"github.com/shipyard-run/shipyard/pkg/utils"
)

// Clients contains clients which are responsible for creating and destrying reources
type Clients struct {
	Docker         clients.Docker
	ContainerTasks clients.ContainerTasks
	Kubernetes     clients.Kubernetes
	Helm           clients.Helm
	HTTP           clients.HTTP
	Nomad          clients.Nomad
	Command        clients.Command
	Logger         hclog.Logger
	Getter         clients.Getter
	Browser        clients.System
	ImageLog       clients.ImageLog
	Connector      clients.Connector
	Parser         *parser.Parser
}

// Engine defines an interface for the Shipyard engine
type Engine interface {
	GetClients() *Clients
	Apply(string) ([]config.Resource, error)

	// ApplyWithVariables applies a configuration file or directory containing
	// configuraiton. Optionally the user can provide a map of variables which the configuraiton
	// uses and / or a file containing variables.
	ApplyWithVariables(path string, variables map[string]string, variablesFile string) ([]config.Resource, error)
	ParseConfig(string) error
	ParseConfigWithVariables(string, map[string]string, string) error
	Destroy(string, bool) error
	ResourceCount() int
	ResourceCountForType(string) int
	Blueprint() *config.Blueprint
}

// EngineImpl is responsible for creating and destroying resources
type EngineImpl struct {
	clients     *Clients
	config      *config.Config
	log         hclog.Logger
	getProvider getProviderFunc
	sync        sync.Mutex
}

// defines a function which is used for generating providers
// enables the replacement in tests to inject mocks
type getProviderFunc func(c config.Resource, cl *Clients) providers.Provider

// GenerateClients creates the various clients for creating and destroying resources
func GenerateClients(l hclog.Logger) (*Clients, error) {
	dc, err := clients.NewDocker()
	if err != nil {
		return nil, err
	}

	kc := clients.NewKubernetes(60*time.Second, l)

	hec := clients.NewHelm(l)

	ec := clients.NewCommand(30*time.Second, l)

	hc := clients.NewHTTP(1*time.Second, l)

	nc := clients.NewNomad(hc, 1*time.Second, l)

	bp := clients.NewGetter(false)

	bc := &clients.SystemImpl{}

	il := clients.NewImageFileLog(utils.ImageCacheLog())

	ct := clients.NewDockerTasks(dc, il, l)

	co := clients.DefaultConnectorOptions()
	cc := clients.NewConnector(co)

	p := parser.New(bp)

	return &Clients{
		ContainerTasks: ct,
		Docker:         dc,
		Kubernetes:     kc,
		Helm:           hec,
		Command:        ec,
		HTTP:           hc,
		Nomad:          nc,
		Logger:         l,
		Getter:         bp,
		Browser:        bc,
		ImageLog:       il,
		Connector:      cc,
		Parser:         p,
	}, nil
}

// New creates a new shipyard engine
func New(l hclog.Logger) (Engine, error) {
	var err error
	e := &EngineImpl{}
	e.log = l
	e.getProvider = generateProviderImpl

	// Set the standard writer to our logger as the DAG uses the standard library log.
	log.SetOutput(l.StandardWriter(&hclog.StandardLoggerOptions{ForceLevel: hclog.Trace}))

	// create the clients
	cl, err := GenerateClients(l)
	if err != nil {
		return nil, err
	}

	e.clients = cl

	return e, nil
}

// GetClients returns the clients from the engine
func (e *EngineImpl) GetClients() *Clients {
	return e.clients
}

// ParseConfig parses the given Shipyard files and creating the resource types but does
// not apply or destroy the resources.
// This function can be used to check the validity of a configuration without making changes
func (e *EngineImpl) ParseConfig(path string) error {
	return e.ParseConfigWithVariables(path, nil, "")
}

// ParseConfigWithVariables parses the given Shipyard files and creating the resource types but does
// not apply or destroy the resources.
// This function can be used to check the validity of a configuration without making changes
func (e *EngineImpl) ParseConfigWithVariables(path string, vars map[string]string, variablesFile string) error {
	_, err := e.readConfig(path, vars, variablesFile)
	if err != nil {
		return err
	}

	return nil
}

// Apply the configuration and create or destroy the resources
func (e *EngineImpl) Apply(path string) ([]config.Resource, error) {
	return e.ApplyWithVariables(path, nil, "")
}

// ApplyWithVariables applies the current config creating the resources
func (e *EngineImpl) ApplyWithVariables(path string, vars map[string]string, variablesFile string) ([]config.Resource, error) {
	// abs paths
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if variablesFile != "" {
		variablesFile, err = filepath.Abs(variablesFile)
		if err != nil {
			return nil, err
		}
	}

	d, err := e.readConfig(path, vars, variablesFile)
	if err != nil {
		return nil, err
	}

	createdResource := []config.Resource{}

	// walk the dag and apply the config
	w := dag.Walker{}
	w.Callback = func(v dag.Vertex) (diags tfdiags.Diagnostics) {

		r, ok := v.(config.Resource)

		// not a resource quit
		if !ok {
			return nil
		}

		// get the provider to create the resource
		p := e.getProvider(r, e.clients)

		if p == nil {
			r.Info().Status = config.Failed
			return diags.Append(fmt.Errorf("Unable to create provider for resource Name: %s, Type: %s", r.Info().Name, r.Info().Type))
		}

		switch r.Info().Status {
		// Normal case for PendingUpdate is do nothing
		// PendingModification causes a resource to be
		// destroyed before created
		case config.PendingModification:
			fallthrough

			// Always attempt to destroy and re-create failed resources
		case config.Failed:
			err = p.Destroy()
			if err != nil {
				r.Info().Status = config.Failed
				return diags.Append(err)
			}

			fallthrough // failed resources should always attempt recreation

		// Create new resources
		case config.PendingCreation:
			createErr := p.Create()
			if createErr != nil {
				r.Info().Status = config.Failed
				return diags.Append(createErr)
			}

		case config.PendingUpdate:
			// do nothing for pending updates

		case config.Disabled:
			// do nothing for disabled updates
		}

		// set the status only if not disabled
		if r.Info().Status != config.Disabled {
			r.Info().Status = config.Applied
		}

		appendResources(&createdResource, r)

		return nil
	}

	w.Update(d)
	tf := w.Wait()
	if tf.Err() != nil {
		err = tf.Err()
	}

	if len(e.config.Resources) > 0 {
		// save the state regardless of error
		jerr := e.config.ToJSON(utils.StatePath())
		if jerr != nil {
			return createdResource, jerr
		}

		return createdResource, err
	}

	return nil, tf.Err()
}

// Destroy the resources defined by the config
func (e *EngineImpl) Destroy(path string, allResources bool) error {
	d, err := e.readConfig(path, nil, "")
	if err != nil {
		return err
	}

	// make sure we destroy everything
	if allResources {
		for _, i := range e.config.Resources {
			if i.Info().Status != config.Disabled {
				i.Info().Status = config.PendingUpdate
			}
		}
	}

	// walk the dag and apply the config
	w := dag.Walker{}
	w.Reverse = true
	w.Callback = func(v dag.Vertex) (diags tfdiags.Diagnostics) {

		// check if the resource needs to be created and if so create
		if r, ok := v.(config.Resource); ok {
			switch r.Info().Status {
			case config.PendingUpdate:
				// do nothing for disabled resources
				if r.Info().Status == config.Disabled {
					r.Info().Status = config.Destroyed
					return nil
				}

				// get the provider to create the resource
				p := e.getProvider(r, e.clients)
				if p == nil {
					r.Info().Status = config.Failed
					return diags.Append(fmt.Errorf("Unable to create provider for resource Name: %s, Type: %s", r.Info().Name, r.Info().Type))
				}

				// execute
				destroyErr := p.Destroy()
				if destroyErr != nil {
					r.Info().Status = config.Failed
					return diags.Append(destroyErr)
				}

				fallthrough
			case config.Disabled:
				// set the status
				r.Info().Status = config.Destroyed
			}
		}

		return nil
	}

	w.Update(d)
	tf := w.Wait()
	if tf.Err() != nil {
		err = tf.Err()
	}

	// remove any destroyed nodes from the state
	cn := config.New()
	for _, i := range e.config.Resources {
		if i.Info().Status != config.Destroyed {
			cn.AddResource(i)
		}
	}

	// save the state regardless of error
	if len(cn.Resources) > 0 {
		err = cn.ToJSON(utils.StatePath())
		if err != nil {
			return err
		}
	} else {
		// if no resources in the state delete
		os.RemoveAll(utils.StatePath())
	}

	return tf.Err()
}

// ResourceCount defines the number of resources in a plan
func (e *EngineImpl) ResourceCount() int {
	return e.config.ResourceCount()
}

// ResourceCountForType returns the count of resources matching the given type
func (e *EngineImpl) ResourceCountForType(t string) int {
	return len(e.config.FindResourcesByType(t))
}

// Blueprint returns the blueprint for the current config
func (e *EngineImpl) Blueprint() *config.Blueprint {
	return e.config.Blueprint
}

func (e *EngineImpl) readConfig(path string, variables map[string]string, variablesFile string) (*dag.AcyclicGraph, error) {
	// create the new config
	cc := config.New()

	// load the existing state
	sc := config.New()
	if _, err := os.Stat(utils.StatePath()); err == nil {
		err := sc.FromJSON(utils.StatePath())
		if err != nil {
			return nil, fmt.Errorf("Error parsing state: %s", err)
		}
	} else {
		e.log.Debug("Statefile does not exist")
	}

	// check to see we have an image cache
	// if not create one
	cache, err := sc.FindResource("docker-cache")
	if err != nil {
		// add a default resource for the docker caching proxy
		proxy := config.NewImageCache("docker-cache")
		sc.AddResource(proxy)

		cache = proxy
	}

	// add the cache to the new config so we can parse networks
	cc.AddResource(cache)

	if path != "" {
		if utils.IsHCLFile(path) {
			err := e.clients.Parser.ParseFile(path, cc, variables, variablesFile)
			if err != nil {
				return nil, err
			}
		} else {
			err := e.clients.Parser.ParseFolder(path, cc, false, "", false, []string{}, variables, variablesFile)
			if err != nil {
				return nil, err
			}
		}

		// if we are loading from files create the deps
		e.clients.Parser.ParseReferences(cc)
	}

	// merge the state and items to be created or deleted
	sc.Merge(cc)

	// set the config
	e.config = sc

	// build a DAG
	d, err := e.config.DoYaLikeDAGs()
	if err != nil {
		return nil, xerrors.Errorf("Unable to create dependency graph: %w", err)
	}

	d.TransitiveReduction()

	err = d.Validate()
	if err != nil {
		return nil, xerrors.Errorf("Unable to validate dependency graph: %w", err)
	}

	return d, nil
}

// generateProviderImpl returns providers grouped together in order of execution
func generateProviderImpl(c config.Resource, cc *Clients) providers.Provider {
	switch c.Info().Type {
	case config.TypeContainer:
		return providers.NewContainer(c.(*config.Container), cc.ContainerTasks, cc.HTTP, cc.Logger)
	case config.TypeContainerIngress:
		return providers.NewContainerIngress(c.(*config.ContainerIngress), cc.ContainerTasks, cc.Logger)
	case config.TypeSidecar:
		return providers.NewContainerSidecar(c.(*config.Sidecar), cc.ContainerTasks, cc.HTTP, cc.Logger)
	case config.TypeDocs:
		return providers.NewDocs(c.(*config.Docs), cc.ContainerTasks, cc.Logger)
	case config.TypeExecRemote:
		return providers.NewRemoteExec(c.(*config.ExecRemote), cc.ContainerTasks, cc.Logger)
	case config.TypeExecLocal:
		return providers.NewExecLocal(c.(*config.ExecLocal), cc.Command, cc.Logger)
	case config.TypeHelm:
		return providers.NewHelm(c.(*config.Helm), cc.Kubernetes, cc.Helm, cc.Getter, cc.Logger)
	case config.TypeIngress:
		return providers.NewIngress(c.(*config.Ingress), cc.ContainerTasks, cc.Connector, cc.Logger)
	case config.TypeImageCache:
		return providers.NewImageCache(c.(*config.ImageCache), cc.ContainerTasks, cc.HTTP, cc.Logger)
	case config.TypeK8sCluster:
		return providers.NewK8sCluster(c.(*config.K8sCluster), cc.ContainerTasks, cc.Kubernetes, cc.HTTP, cc.Connector, cc.Logger)
	case config.TypeK8sConfig:
		return providers.NewK8sConfig(c.(*config.K8sConfig), cc.Kubernetes, cc.Logger)
	case config.TypeK8sIngress:
		return providers.NewK8sIngress(c.(*config.K8sIngress), cc.ContainerTasks, cc.Logger)
	case config.TypeNomadCluster:
		return providers.NewNomadCluster(c.(*config.NomadCluster), cc.ContainerTasks, cc.Nomad, cc.Logger)
	case config.TypeNomadIngress:
		return providers.NewNomadIngress(c.(*config.NomadIngress), cc.ContainerTasks, cc.Logger)
	case config.TypeNomadJob:
		return providers.NewNomadJob(c.(*config.NomadJob), cc.Nomad, cc.Logger)
	case config.TypeNetwork:
		return providers.NewNetwork(c.(*config.Network), cc.Docker, cc.Logger)
	case config.TypeOutput:
		return providers.NewNull(c.Info(), cc.Logger)
	case config.TypeTemplate:
		return providers.NewTemplate(c.(*config.Template), cc.Logger)
	}

	return nil
}

var crMutex = sync.Mutex{}

// appends item to the resources slice in a thread safe way
func appendResources(cr *[]config.Resource, r config.Resource) {
	crMutex.Lock()
	defer crMutex.Unlock()

	*cr = append(*cr, r)
}
