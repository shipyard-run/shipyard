package parser

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gernest/front"
	"github.com/hashicorp/hcl2/gohcl"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcl/hclsyntax"
	"github.com/hashicorp/hcl2/hclparse"
	"github.com/shipyard-run/shipyard/pkg/clients"
	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/utils"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"golang.org/x/xerrors"
)

var NewGetter = func(forceUpdate bool) clients.Getter {
	return clients.NewGetter(forceUpdate)
}

type ResourceTypeNotExistError struct {
	Type string
	File string
}

func (r ResourceTypeNotExistError) Error() string {
	return fmt.Sprintf("Resource type %s defined in file %s, does not exist. Please check the documentation for supported resources. We love PRs if you would like to create a resource of this type :)", r.Type, r.File)
}

type Parser struct {
	ctx    *hcl.EvalContext
	getter clients.Getter
}

func New(g clients.Getter) *Parser {
	return &Parser{getter: g}
}

func (p *Parser) ParseFile(file string, c *config.Config, variables map[string]string, variablesFile string) error {
	p.ctx = buildContext()

	return p.parseFile(file, c, variables, variablesFile)
}

// ParseFolder for Resource, Blueprint, and Variable files
// forceUpdate always downloads a new copy of remote modules, if set to false, cached copy is used.
// onlyResources parameter allows you to specify that the parser
// moduleName is the name of the module, this should be set to a blank string for the root module
// disabled sets the disabled flag on all resources, this is used when parsing a module that
//  has the disabled flag set
// only reads resource files and will ignore Blueprint and Variable files.
// This is useful when recursively parsing such as when reading Modules
func (p *Parser) ParseFolder(
	folder string,
	c *config.Config,
	onlyResources bool,
	moduleName string,
	disabled bool,
	dependsOn []string,
	variables map[string]string,
	variablesFile string) error {

	p.ctx = buildContext()
	return p.parseFolder(
		folder,
		c,
		onlyResources,
		moduleName,
		disabled,
		dependsOn,
		variables,
		variablesFile,
	)
}

// LoadValuesFile loads variable values from a file
func (p *Parser) LoadValuesFile(path string) error {
	hp := hclparse.NewParser()

	f, diag := hp.ParseHCLFile(path)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	// add the file functions to the context with a reference to the
	// current file
	p.ctx.Functions["file_path"] = getFilePathFunc(path)
	p.ctx.Functions["file_dir"] = getFileDirFunc(path)
	p.ctx.Functions["file"] = getFileContentFunc(path)

	attrs, _ := f.Body.JustAttributes()
	for name, attr := range attrs {
		val, _ := attr.Expr.Value(p.ctx)

		p.setContextVariable(name, val)
	}

	return nil
}

// SetVariables allow variables to be set from a collection or environment variables
// Precedence should be file, env, vars
func (p *Parser) SetVariables(vars map[string]string) {
	// first any vars defined as environment variables
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "SY_VAR_") {
			parts := strings.Split(e, "=")
			p.setContextVariable(strings.Replace(parts[0], "SY_VAR_", "", -1), parts[1])
		}
	}

	// then set vars
	for k, v := range vars {
		p.setContextVariable(k, v)
	}
}

func (p *Parser) parseFile(file string, c *config.Config, variables map[string]string, variablesFile string) error {
	p.SetVariables(variables)
	if variablesFile != "" {
		err := p.LoadValuesFile(variablesFile)
		if err != nil {
			return err
		}
	}

	err := p.parseVariableFile(file, c)
	if err != nil {
		return err
	}

	err = p.parseHCLFile(file, c, "", false, []string{})
	if err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseFolder(
	folder string,
	c *config.Config,
	onlyResources bool,
	moduleName string,
	disabled bool,
	dependsOn []string,
	variables map[string]string,
	variablesFile string) error {

	abs, _ := filepath.Abs(folder)

	// load the variables from the root of the blueprint
	if !onlyResources {
		variableFiles, err := filepath.Glob(path.Join(abs, "*.vars"))
		if err != nil {
			return err
		}

		for _, f := range variableFiles {
			err := p.LoadValuesFile(f)
			if err != nil {
				return err
			}
		}

		// load variables from any custom files set on the command line
		if variablesFile != "" {
			err := p.LoadValuesFile(variablesFile)
			if err != nil {
				return err
			}
		}

		// setup any variables which are passed as environment variables or in the collection
		p.SetVariables(variables)

		// pick up the blueprint file
		yardFilesHCL, err := filepath.Glob(path.Join(abs, "*.yard"))
		if err != nil {
			return err
		}

		yardFilesMD, err := filepath.Glob(path.Join(abs, "README.md"))
		if err != nil {
			return err
		}

		yardFiles := []string{}
		yardFiles = append(yardFiles, yardFilesHCL...)
		yardFiles = append(yardFiles, yardFilesMD...)

		if len(yardFiles) > 0 {
			err := p.parseYardFile(yardFiles[0], c)
			if err != nil {
				return err
			}
		}
	}

	// We need to do a two pass parsing, first we check if there are any
	// default variables which should be added to the collection
	err := p.parseVariables(abs, c)
	if err != nil {
		return err
	}

	// Parse Resource files from the current folder
	err = p.parseResources(abs, c, moduleName, disabled, dependsOn)
	if err != nil {
		return err
	}

	// Finally parse the outputs
	err = p.parseOutputs(abs, disabled, c)
	if err != nil {
		return err
	}

	return nil
}

// ParseYardFile parses a blueprint configuration file
func (p *Parser) parseYardFile(file string, c *config.Config) error {
	if filepath.Ext(file) == ".yard" {
		return p.parseYardHCL(file, c)
	}

	return p.parseYardMarkdown(file, c)
}

// ParseVariableFile parses a config file for variables
func (p *Parser) parseVariableFile(file string, c *config.Config) error {
	hp := hclparse.NewParser()
	p.ctx.Functions["file_path"] = getFilePathFunc(file)
	p.ctx.Functions["file_dir"] = getFileDirFunc(file)
	p.ctx.Functions["file"] = getFileContentFunc(file)

	f, diag := hp.ParseHCLFile(file)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return errors.New("Error getting body")
	}

	for _, b := range body.Blocks {
		switch b.Type {
		case string(config.TypeVariable):
			v := config.NewVariable(b.Labels[0])

			err := p.decodeBody(file, b, v)
			if err != nil {
				return err
			}

			val, _ := v.Default.(*hcl.Attribute).Expr.Value(p.ctx)
			p.setContextVariableIfMissing(v.Name, val)
		}
	}

	return nil
}

// parseHCLFile parses a config file and adds it to the config
func (p *Parser) parseHCLFile(file string, c *config.Config, moduleName string, disabled bool, dependsOn []string) error {
	parser := hclparse.NewParser()
	p.ctx.Functions["file_path"] = getFilePathFunc(file)
	p.ctx.Functions["file_dir"] = getFileDirFunc(file)
	p.ctx.Functions["file"] = getFileContentFunc(file)

	f, diag := parser.ParseHCLFile(file)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return errors.New("Error getting body")
	}

	for _, b := range body.Blocks {
		//fmt.Printf("Parsing: %s, type: %s\n", file, b.Type)

		switch b.Type {
		case string(config.TypeVariable):
			// do nothing this is only here to
			// stop the resource not found error
			continue

		case string(config.TypeOutput):
			// do nothing this is only here to
			// stop the resource not found error
			continue

		case string(config.TypeK8sCluster):
			cl := config.NewK8sCluster(b.Labels[0])
			cl.Info().Module = moduleName
			cl.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, cl)
			if err != nil {
				return err
			}

			// Process volumes
			// make sure mount paths are absolute
			for i, v := range cl.Volumes {
				cl.Volumes[i].Source = ensureAbsolute(v.Source, file)
			}

			setDisabled(cl, disabled)

			err = c.AddResource(cl)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeK8sConfig):
			h := config.NewK8sConfig(b.Labels[0])
			h.Info().Module = moduleName
			h.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, h)
			if err != nil {
				return err
			}

			// make all the paths absolute
			for i, p := range h.Paths {
				h.Paths[i] = ensureAbsolute(p, file)
			}

			setDisabled(h, disabled)

			err = c.AddResource(h)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeHelm):
			h := config.NewHelm(b.Labels[0])
			h.Info().Module = moduleName
			h.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, h)
			if err != nil {
				return err
			}

			// if ChartName is not set use the name of the chart use the name of the
			// resource
			if h.ChartName == "" {
				h.ChartName = h.Name
			}

			// only set absolute if is local folder
			if h.Chart != "" && utils.IsLocalFolder(ensureAbsolute(h.Chart, file)) {
				h.Chart = ensureAbsolute(h.Chart, file)
			}

			if h.Values != "" {
				h.Values = ensureAbsolute(h.Values, file)
			}

			setDisabled(h, disabled)

			err = c.AddResource(h)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeK8sIngress):
			i := config.NewK8sIngress(b.Labels[0])
			i.Info().Module = moduleName
			i.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, i)
			if err != nil {
				return err
			}

			setDisabled(i, disabled)

			err = c.AddResource(i)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeNomadCluster):
			cl := config.NewNomadCluster(b.Labels[0])
			cl.Info().Module = moduleName
			cl.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, cl)
			if err != nil {
				return err
			}

			// Process volumes
			// make sure mount paths are absolute
			for i, v := range cl.Volumes {
				cl.Volumes[i].Source = ensureAbsolute(v.Source, file)
			}

			setDisabled(cl, disabled)

			err = c.AddResource(cl)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeNomadJob):
			h := config.NewNomadJob(b.Labels[0])
			h.Info().Module = moduleName
			h.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, h)
			if err != nil {
				return err
			}

			// make all the paths absolute
			for i, p := range h.Paths {
				h.Paths[i] = ensureAbsolute(p, file)
			}

			setDisabled(h, disabled)

			err = c.AddResource(h)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeNomadIngress):
			i := config.NewNomadIngress(b.Labels[0])
			i.Info().Module = moduleName
			i.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, i)
			if err != nil {
				return err
			}

			setDisabled(i, disabled)

			err = c.AddResource(i)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeNetwork):
			n := config.NewNetwork(b.Labels[0])
			n.Info().Module = moduleName
			n.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, n)
			if err != nil {
				return err
			}

			setDisabled(n, disabled)

			err = c.AddResource(n)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

			// always add this network as a dependency of the image cache
			ics := c.FindResourcesByType(string(config.TypeImageCache))
			if ics != nil && len(ics) == 1 {
				ic := ics[0].(*config.ImageCache)
				ic.DependsOn = append(ic.DependsOn, "network."+n.Name)
			}

		case string(config.TypeIngress):
			i := config.NewIngress(b.Labels[0])
			i.Info().Module = moduleName
			i.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, i)
			if err != nil {
				return err
			}

			setDisabled(i, disabled)

			err = c.AddResource(i)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeContainer):
			co := config.NewContainer(b.Labels[0])
			co.Info().Module = moduleName
			co.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, co)
			if err != nil {
				return err
			}

			// process volumes
			for i, v := range co.Volumes {
				// make sure mount paths are absolute when type is bind
				if v.Type == "" || v.Type == "bind" {
					co.Volumes[i].Source = ensureAbsolute(v.Source, file)
				}
			}

			// make sure build paths are absolute
			if co.Build != nil {
				co.Build.Context = ensureAbsolute(co.Build.Context, file)
			}

			setDisabled(co, disabled)

			err = c.AddResource(co)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeContainerIngress):
			i := config.NewContainerIngress(b.Labels[0])
			i.Info().Module = moduleName
			i.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, i)
			if err != nil {
				return err
			}

			setDisabled(i, disabled)

			err = c.AddResource(i)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeSidecar):
			s := config.NewSidecar(b.Labels[0])
			s.Info().Module = moduleName
			s.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, s)
			if err != nil {
				return err
			}

			for i, v := range s.Volumes {
				s.Volumes[i].Source = ensureAbsolute(v.Source, file)
			}

			setDisabled(s, disabled)

			err = c.AddResource(s)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeDocs):
			do := config.NewDocs(b.Labels[0])
			do.Info().Module = moduleName
			do.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, do)
			if err != nil {
				return err
			}

			do.Path = ensureAbsolute(do.Path, file)

			setDisabled(do, disabled)

			c.AddResource(do)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeExecLocal):
			h := config.NewExecLocal(b.Labels[0])
			h.Info().Module = moduleName
			h.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, h)
			if err != nil {
				return err
			}

			setDisabled(h, disabled)

			err = c.AddResource(h)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeExecRemote):
			h := config.NewExecRemote(b.Labels[0])
			h.Info().Module = moduleName
			h.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, h)
			if err != nil {
				return err
			}

			// process volumes
			// make sure mount paths are absolute
			for i, v := range h.Volumes {
				h.Volumes[i].Source = ensureAbsolute(v.Source, file)
			}

			setDisabled(h, disabled)

			err = c.AddResource(h)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeTemplate):
			i := config.NewTemplate(b.Labels[0])
			i.Info().Module = moduleName
			i.Info().DependsOn = dependsOn

			err := p.decodeBody(file, b, i)
			if err != nil {
				return err
			}

			i.Destination = ensureAbsolute(i.Destination, file)

			setDisabled(i, disabled)

			err = c.AddResource(i)
			if err != nil {
				return fmt.Errorf(
					"Unable to add resource %s.%s in file %s: %s",
					b.Type,
					b.Labels[0],
					file,
					err,
				)
			}

		case string(config.TypeModule):
			moduleName := b.Labels[0]
			m := config.NewModule(moduleName)
			m.Info().Module = moduleName

			err := p.decodeBody(file, b, m)
			if err != nil {
				return err
			}

			// import the source files for this module
			if !utils.IsLocalFolder(ensureAbsolute(m.Source, file)) {
				// get the details
				dst := utils.GetBlueprintLocalFolder(m.Source)
				err := p.getFiles(m.Source, dst)
				if err != nil {
					return err
				}

				// set the source to the local folder
				m.Source = dst
			}

			// set the absolute path
			m.Source = ensureAbsolute(m.Source, file)

			// if the module is disabled ensure
			setDisabled(m, disabled)

			// recursively parse references for the module
			// ensure we do load the values which might be in module folders
			err = p.parseFolder(m.Source, c, true, moduleName, m.Disabled, m.Depends, nil, "")
			if err != nil {
				return err
			}

			// modules will reset the context file path as they recurse
			// into other folders. They should have a separate context but
			// for now just reset the file path to ensure any other resources
			// parsed after the module have the correct path
			p.ctx.Functions["file_path"] = getFilePathFunc(file)
			p.ctx.Functions["file_dir"] = getFileDirFunc(file)
			p.ctx.Functions["file"] = getFileContentFunc(file)

		default:
			return ResourceTypeNotExistError{string(b.Type), file}
		}
	}

	return nil
}

// ParseReferences links the object references in config elements
func (p *Parser) ParseReferences(c *config.Config) error {
	for _, r := range c.Resources {
		switch r.Info().Type {
		case config.TypeContainer:
			c := r.(*config.Container)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeContainerIngress:
			c := r.(*config.ContainerIngress)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Target)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeSidecar:
			c := r.(*config.Sidecar)
			c.DependsOn = append(c.DependsOn, c.Target)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeDocs:
			c := r.(*config.Docs)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeExecRemote:
			c := r.(*config.ExecRemote)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}

			c.DependsOn = append(c.DependsOn, c.Depends...)

			// target is optional
			if c.Target != "" {
				c.DependsOn = append(c.DependsOn, c.Target)
			}

		case config.TypeExecLocal:
			c := r.(*config.ExecLocal)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeTemplate:
			c := r.(*config.Template)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeIngress:
			c := r.(*config.Ingress)
			if c.Source.Config.Cluster != "" {
				c.DependsOn = append(c.DependsOn, c.Source.Config.Cluster)
			}

			if c.Destination.Config.Cluster != "" {
				c.DependsOn = append(c.DependsOn, c.Destination.Config.Cluster)
			}

			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeK8sCluster:
			c := r.(*config.K8sCluster)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Depends...)

			// always add a dependency of the cache as this is
			// required by all clusters
			c.DependsOn = append(c.DependsOn, fmt.Sprintf("%s.%s", config.TypeImageCache, utils.CacheResourceName))

		case config.TypeHelm:
			c := r.(*config.Helm)
			c.DependsOn = append(c.DependsOn, c.Cluster)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeK8sConfig:
			c := r.(*config.K8sConfig)
			c.DependsOn = append(c.DependsOn, c.Cluster)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeK8sIngress:
			c := r.(*config.K8sIngress)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Cluster)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeNomadCluster:
			c := r.(*config.NomadCluster)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Depends...)
			// always add a dependency of the cache as this is
			// required by all clusters
			c.DependsOn = append(c.DependsOn, fmt.Sprintf("%s.%s", config.TypeImageCache, utils.CacheResourceName))

		case config.TypeNomadIngress:
			c := r.(*config.NomadIngress)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Cluster)
			c.DependsOn = append(c.DependsOn, c.Depends...)

		case config.TypeNomadJob:
			c := r.(*config.NomadJob)
			c.DependsOn = append(c.DependsOn, c.Cluster)
		}
	}

	return nil
}

func (p *Parser) parseVariables(abs string, c *config.Config) error {
	files, err := filepath.Glob(path.Join(abs, "*.hcl"))
	if err != nil {
		return err
	}

	for _, f := range files {
		err := p.parseVariableFile(f, c)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) parseOutputs(abs string, disabled bool, c *config.Config) error {
	files, err := filepath.Glob(path.Join(abs, "*.hcl"))
	if err != nil {
		return err
	}

	for _, f := range files {
		err := p.parseOutputFile(f, disabled, c)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) parseOutputFile(file string, disabled bool, c *config.Config) error {
	parser := hclparse.NewParser()
	p.ctx.Functions["file_path"] = getFilePathFunc(file)
	p.ctx.Functions["file_dir"] = getFileDirFunc(file)
	p.ctx.Functions["file"] = getFileContentFunc(file)

	f, diag := parser.ParseHCLFile(file)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return errors.New("Error getting body")
	}

	for _, b := range body.Blocks {
		switch b.Type {
		case string(config.TypeOutput):
			v := config.NewOutput(b.Labels[0])

			err := p.decodeBody(file, b, v)
			if err != nil {
				return err
			}

			setDisabled(v, disabled)

			c.AddResource(v)
		}
	}

	return nil
}

func (p *Parser) parseResources(abs string, c *config.Config, moduleName string, disabled bool, dependsOn []string) error {
	files, err := filepath.Glob(path.Join(abs, "*.hcl"))
	if err != nil {
		return err
	}

	for _, f := range files {
		err := p.parseHCLFile(f, c, moduleName, disabled, dependsOn)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) setContextVariable(key string, value interface{}) {
	valMap := map[string]cty.Value{}

	// get the existing map
	if m, ok := p.ctx.Variables["var"]; ok {
		valMap = m.AsValueMap()
	}

	switch v := value.(type) {
	case string:
		valMap[key] = cty.StringVal(v)
		//fmt.Println("Adding String Var", key, v)
	case cty.Value:
		valMap[key] = v
		//fmt.Println("Adding Var", key, v)
	}

	p.ctx.Variables["var"] = cty.ObjectVal(valMap)
}

func (p *Parser) setContextVariableIfMissing(key string, value interface{}) {
	if m, ok := p.ctx.Variables["var"]; ok {
		if _, ok := m.AsValueMap()[key]; ok {
			return
		}
	}

	p.setContextVariable(key, value)
}

func (p *Parser) parseYardHCL(file string, c *config.Config) error {
	hp := hclparse.NewParser()

	f, diag := hp.ParseHCLFile(file)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return errors.New("Error getting body")
	}

	bp := &config.Blueprint{}

	diag = gohcl.DecodeBody(body, p.ctx, bp)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	c.Blueprint = bp

	return nil
}

// parseYardMarkdown extracts the blueprint information from the frontmatter
// when a blueprint file is of type markdown
func (p *Parser) parseYardMarkdown(file string, c *config.Config) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}

	m := front.NewMatter()
	m.Handle("---", front.YAMLHandler)

	fr, body, err := m.Parse(f)
	if err != nil && err != front.ErrIsEmpty {
		fmt.Println("Error parsing README.md", err)
		return nil
	}

	bp := &config.Blueprint{}
	bp.HealthCheckTimeout = "30s"

	// set the default health check

	if a, ok := fr["author"].(string); ok {
		bp.Author = a
	}

	if a, ok := fr["title"].(string); ok {
		bp.Title = a
	}

	if a, ok := fr["slug"].(string); ok {
		bp.Slug = a
	}

	if a, ok := fr["browser_windows"].(string); ok {
		bp.BrowserWindows = strings.Split(a, ",")
	}

	if a, ok := fr["health_check_timeout"].(string); ok {
		bp.HealthCheckTimeout = a
	}

	if a, ok := fr["shipyard_version"].(string); ok {
		bp.ShipyardVersion = a
	}

	if envs, ok := fr["env"].([]interface{}); ok {
		bp.Environment = []config.KV{}
		for _, e := range envs {
			parts := strings.Split(e.(string), "=")
			if len(parts) == 2 {
				bp.Environment = append(bp.Environment, config.KV{Key: parts[0], Value: parts[1]})
			}
		}
	}

	bp.Intro = body

	c.Blueprint = bp
	return nil
}

func (p *Parser) decodeBody(path string, b *hclsyntax.Block, res interface{}) error {
	// add the current file path to the context.
	// this allows any functions which require absolute paths to be able to
	// build them from relative paths.
	p.ctx.Variables["path"] = cty.StringVal(path)

	diag := gohcl.DecodeBody(b.Body, p.ctx, res)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	return nil
}

func buildContext() *hcl.EvalContext {
	var EnvFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "env",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(os.Getenv(args[0].AsString())), nil
		},
	})

	var HomeFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(utils.HomeFolder()), nil
		},
	})

	var ShipyardFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(utils.ShipyardHome()), nil
		},
	})

	var DockerIPFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(utils.GetDockerIP()), nil
		},
	})

	var DockerHostFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(utils.GetDockerHost()), nil
		},
	})

	var ShipyardIPFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			ip, _ := utils.GetLocalIPAndHostname()
			return cty.StringVal(ip), nil
		},
	})

	var KubeConfigFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "k8s_config",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			_, kcp, _ := utils.CreateKubeConfigPath(args[0].AsString())
			return cty.StringVal(kcp), nil
		},
	})

	var KubeConfigDockerFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "k8s_config_docker",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			_, _, kcp := utils.CreateKubeConfigPath(args[0].AsString())
			return cty.StringVal(kcp), nil
		},
	})

	var DataFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "path",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(utils.GetDataFolder(args[0].AsString())), nil
		},
	})

	var ClusterAPIFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "name",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			conf, _ := utils.GetClusterConfig(args[0].AsString())

			return cty.StringVal(conf.APIAddress(utils.LocalContext)), nil
		},
	})

	ctx := &hcl.EvalContext{
		Functions: map[string]function.Function{},
		Variables: map[string]cty.Value{},
	}

	ctx.Functions["env"] = EnvFunc
	ctx.Functions["k8s_config"] = KubeConfigFunc
	ctx.Functions["k8s_config_docker"] = KubeConfigDockerFunc
	ctx.Functions["home"] = HomeFunc
	ctx.Functions["shipyard"] = ShipyardFunc
	ctx.Functions["data"] = DataFunc
	ctx.Functions["docker_ip"] = DockerIPFunc
	ctx.Functions["docker_host"] = DockerHostFunc
	ctx.Functions["shipyard_ip"] = ShipyardIPFunc
	ctx.Functions["cluster_api"] = ClusterAPIFunc

	// the functions file_path and file_dir are added dynamically when processing a file
	// this is because the need a reference to the current file

	return ctx
}

// ensureAbsolute ensure that the given path is either absolute or
// if relative is converted to abasolute based on the path of the config
func ensureAbsolute(path, file string) string {
	// if the file starts with a / and we are on windows
	// we should treat this as absolute
	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
		return path
	}

	if filepath.IsAbs(path) {
		return path
	}

	// path is relative so make absolute using the current file path as base
	file, _ = filepath.Abs(file)
	baseDir := filepath.Dir(file)
	return filepath.Join(baseDir, path)
}

func (p *Parser) getFiles(source, dest string) error {
	err := p.getter.Get(source, dest)
	if err != nil {
		return xerrors.Errorf("unable to fetch files from %s: %w", source, err)
	}

	return nil
}

// setDisabled sets the disabled flag on a resource when the
// parent is disabled
func setDisabled(r config.Resource, parentDisabled bool) {
	if parentDisabled {
		r.Info().Disabled = true
	}

	// when the resource is disabled set the status
	// so the engine will not create or delete it
	if r.Info().Disabled {
		r.Info().Status = "disabled"
	}
}

func getFilePathFunc(path string) function.Function {
	return function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			s, err := filepath.Abs(path)
			return cty.StringVal(s), err
		},
	})
}

func getFileDirFunc(path string) function.Function {
	return function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			s, err := filepath.Abs(path)

			return cty.StringVal(filepath.Dir(s)), err
		},
	})
}

func getFileContentFunc(path string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "path",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			// conver the file path to an absolute
			fp := ensureAbsolute(args[0].AsString(), path)

			// read the contents of the file
			d, err := ioutil.ReadFile(fp)
			if err != nil {
				return cty.StringVal(""), err
			}

			return cty.StringVal(string(d)), nil
		},
	})
}
