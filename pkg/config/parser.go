package config

// TODO how do we deal with multiple stanza with the same name

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/hashicorp/hcl2/gohcl"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcl/hclsyntax"
	"github.com/hashicorp/hcl2/hclparse"
	"github.com/shipyard-run/shipyard/pkg/utils"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var ctx *hcl.EvalContext

// ParseFolder for config entries
func ParseFolder(folder string, c *Config) error {
	ctx = buildContext()

	abs, _ := filepath.Abs(folder)

	// pick up the blueprint file
	yardFiles, err := filepath.Glob(path.Join(abs, "*.yard"))
	if err != nil {
		fmt.Println("err")
		return err
	}

	if len(yardFiles) > 0 {
		err := ParseYardFile(yardFiles[0], c)
		if err != nil {
			fmt.Println("err")
			return err
		}
	}

	// load files from the current folder
	files, err := filepath.Glob(path.Join(abs, "*.hcl"))
	if err != nil {
		fmt.Println("err")
		return err
	}

	// sub folders
	filesDir, err := filepath.Glob(path.Join(abs, "**/*.hcl"))
	if err != nil {
		fmt.Println("err")
		return err
	}

	files = append(files, filesDir...)

	for _, f := range files {
		err := ParseHCLFile(f, c)
		if err != nil {
			return err
		}
	}

	return nil
}

// ParseYardFile parses a blueprint configuration file
func ParseYardFile(file string, c *Config) error {
	parser := hclparse.NewParser()

	f, diag := parser.ParseHCLFile(file)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return errors.New("Error getting body")
	}

	bp := &Blueprint{}

	diag = gohcl.DecodeBody(body, ctx, bp)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	c.Blueprint = bp

	return nil
}

// ParseHCLFile parses a config file and adds it to the config
func ParseHCLFile(file string, c *Config) error {
	parser := hclparse.NewParser()

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
		case string(TypeK8sCluster):
			cl := NewK8sCluster(b.Labels[0])

			err := decodeBody(b, cl)
			if err != nil {
				return err
			}

			c.AddResource(cl)
		case string(TypeNomadCluster):
			cl := NewNomadCluster(b.Labels[0])

			err := decodeBody(b, cl)
			if err != nil {
				return err
			}

			c.AddResource(cl)

		case string(TypeNetwork):
			if b.Labels[0] == "wan" {
				return ErrorWANExists
			}

			n := NewNetwork(b.Labels[0])

			err := decodeBody(b, n)
			if err != nil {
				return err
			}

			c.AddResource(n)

		case string(TypeHelm):
			h := NewHelm(b.Labels[0])

			err := decodeBody(b, h)
			if err != nil {
				return err
			}

			h.Chart = ensureAbsolute(h.Chart, file)
			h.Values = ensureAbsolute(h.Values, file)

			c.AddResource(h)

		case string(TypeK8sConfig):
			h := NewK8sConfig(b.Labels[0])

			err := decodeBody(b, h)
			if err != nil {
				return err
			}

			// make all the paths absolute
			for i, p := range h.Paths {
				h.Paths[i] = ensureAbsolute(p, file)
			}

			c.AddResource(h)

		case string(TypeIngress):
			i := NewIngress(b.Labels[0])

			err := decodeBody(b, i)
			if err != nil {
				return err
			}

			c.AddResource(i)

		case string(TypeContainer):
			co := NewContainer(b.Labels[0])

			err := decodeBody(b, co)
			if err != nil {
				return err
			}

			// process volumes
			// make sure mount paths are absolute
			for i, v := range co.Volumes {
				co.Volumes[i].Source = ensureAbsolute(v.Source, file)
			}

			c.AddResource(co)

		case string(TypeDocs):
			do := NewDocs(b.Labels[0])

			err := decodeBody(b, do)
			if err != nil {
				return err
			}

			do.Path = ensureAbsolute(do.Path, file)

			c.AddResource(do)

		case string(TypeExecLocal):
			h := NewExecLocal(b.Labels[0])

			err := decodeBody(b, h)
			if err != nil {
				return err
			}

			h.Script = ensureAbsolute(h.Script, file)

			c.AddResource(h)

		case string(TypeExecRemote):
			h := NewExecRemote(b.Labels[0])

			err := decodeBody(b, h)
			if err != nil {
				return err
			}

			if h.Script != "" {
				h.Script = ensureAbsolute(h.Script, file)
			}

			// process volumes
			// make sure mount paths are absolute
			for i, v := range h.Volumes {
				h.Volumes[i].Source = ensureAbsolute(v.Source, file)
			}

			c.AddResource(h)
		}
	}

	return nil
}

// ParseReferences links the object references in config elements
func ParseReferences(c *Config) error {
	for _, r := range c.Resources {
		switch r.Info().Type {
		case TypeContainer:
			c := r.(*Container)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}

		case TypeDocs:
			c := r.(*Docs)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
		case TypeExecRemote:
			c := r.(*ExecRemote)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}

			c.DependsOn = append(c.DependsOn, c.Depends...)

			// target is optional
			if c.Target != "" {
				c.DependsOn = append(c.DependsOn, c.Target)
			}
		case TypeHelm:
			c := r.(*Helm)
			c.DependsOn = append(c.DependsOn, c.Cluster)

		case TypeK8sConfig:
			c := r.(*K8sConfig)
			c.DependsOn = append(c.DependsOn, c.Cluster)

		case TypeIngress:
			c := r.(*Ingress)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
			c.DependsOn = append(c.DependsOn, c.Target)
		case TypeK8sCluster:
			c := r.(*K8sCluster)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
		case TypeNomadCluster:
			c := r.(*NomadCluster)
			for _, n := range c.Networks {
				c.DependsOn = append(c.DependsOn, n.Name)
			}
		}
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
			_, _, kcp := utils.CreateKubeConfigPath(args[0].AsString())
			return cty.StringVal(kcp), nil
		},
	})

	ctx := &hcl.EvalContext{
		Functions: map[string]function.Function{},
	}
	ctx.Functions["env"] = EnvFunc
	ctx.Functions["k8s_config"] = KubeConfigFunc

	return ctx
}

func decodeBody(b *hclsyntax.Block, p interface{}) error {
	diag := gohcl.DecodeBody(b.Body, ctx, p)
	if diag.HasErrors() {
		return errors.New(diag.Error())
	}

	return nil
}

// ensureAbsolute ensure that the given path is either absolute or
// if relative is converted to abasolute based on the path of the config
func ensureAbsolute(path, file string) string {
	if filepath.IsAbs(path) {
		return path
	}

	// path is relative so make absolute using the current file path as base
	file, _ = filepath.Abs(file)
	baseDir := filepath.Dir(file)
	return filepath.Join(baseDir, path)
}
