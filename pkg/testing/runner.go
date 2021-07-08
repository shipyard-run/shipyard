package testing

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/hashicorp/go-hclog"
	"github.com/shipyard-run/shipyard/pkg/shipyard"
)

type Runner interface {
	// SetOutputWriter sets the writer used for output
	// defaults to os.StdOut
	SetOutputWriter(w io.Writer)
	// BeforeSuite runs the given function before the suite of tests runs
	BeforeSuite(func() error)
	// AfterSuite runs the given function after the suite of tests has run
	AfterSuite(func() error)
	// Allows a function to be executed before the Scenario runs
	BeforeScenario(func() error)
	// Allows a function to be executed after the Scenario runs
	AfterScenario(func() error)
	// RegsiterStep registers a custom step
	RegisterStep(expr string, stepFunc interface{})
	// Run the tests
	Run() error
}

// goDogRunner is an implementation of Runner for the GoDog Cucumber tool
type goDogRunner struct {
	config         *Config
	engine         shipyard.Engine
	output         *os.File
	engineLog      *bytes.Buffer
	beforeScenario func() error
	afterScenario  func() error
	beforeSuite    func() error
	afterSuite     func() error
	customSteps    map[string]interface{}
}

// NewRunner creates a new Runner with the given configuration
func NewRunner(config *Config) Runner {
	engineLog := &bytes.Buffer{}

	lo := &hclog.LoggerOptions{
		Level:  hclog.Debug,
		Output: engineLog,
	}

	hl := hclog.New(lo)

	engine, err := shipyard.New(hl)
	if err != nil {
		panic(err)
	}

	return &goDogRunner{
		config,
		engine,
		os.Stdout,
		engineLog,
		nil,
		nil,
		nil,
		nil,
		map[string]interface{}{},
	}
}

func (gd *goDogRunner) SetOutputWriter(w io.Writer) {}

func (gd *goDogRunner) BeforeSuite(bs func() error) {
	gd.beforeSuite = bs
}

func (gd *goDogRunner) AfterSuite(as func() error) {
	gd.afterSuite = as
}

func (gd *goDogRunner) BeforeScenario(bs func() error) {
	gd.beforeScenario = bs
}

func (gd *goDogRunner) AfterScenario(as func() error) {
	gd.afterScenario = as
}

func (gd *goDogRunner) RegisterStep(expr string, stepFunc interface{}) {
	gd.customSteps[expr] = stepFunc
}

func (gd *goDogRunner) Run() error {
	var opts = &godog.Options{
		Format: "pretty",
		Output: colors.Colored(gd.output),
		Paths:  []string{gd.config.FeaturesPath},
		Tags:   strings.Join(gd.config.Tags, ","),
	}

	status := godog.TestSuite{
		Name:                "Shipyard test",
		ScenarioInitializer: gd.initializeSuite,
		Options:             opts,
	}.Run()

	if status == 1 {
		return fmt.Errorf("Error running test suite")
	}

	if gd.afterSuite != nil {
		gd.afterSuite()
	}

	return nil
}

func (gd *goDogRunner) initializeSuite(ctx *godog.ScenarioContext) {
	ctx.BeforeScenario(func(gs *godog.Scenario) {
		if gd.beforeScenario != nil {
			gd.beforeScenario()
		}
	})

	ctx.AfterScenario(func(gs *godog.Scenario, err error) {
		if gd.afterScenario != nil {
			gd.afterScenario()
		}

		gd.engine.Destroy("", true)

		if err != nil {
			fmt.Fprintln(gd.output, gd.engineLog.String())
		}
	})

	// register default steps
	ctx.Step(`^I have a running blueprint$`, gd.iRunApply)
	ctx.Step(`^I have a running blueprint at path "([^"]*)"$`, gd.iRunApplyAtPath)

	// register custom steps
	for k, v := range gd.customSteps {
		ctx.Step(k, v)
	}

	if gd.beforeSuite != nil {
		gd.beforeSuite()
	}
}
