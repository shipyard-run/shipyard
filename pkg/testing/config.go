package testing

import (
	"flag"
	"os"
	"strings"
)

var flagVariables arrayFlags
var flagTags string

func init() {
	flag.Var(&flagVariables, "var", "")
	flag.StringVar(&flagTags, "tags", "", "")
}

// Config allows the configuration of the test runners
type Config struct {
	// Specifies if resources should be created when Runner.Run() is called
	CreateResources bool
	// Specifies if resources should be destroyed when Runner.Run() exits
	DestroyResources bool
	// Log level for output [info,debug,trace]
	LogLevel string
	// Shipyard variables to set for the run, these variables
	// take precedence over any set in the feature
	Variables map[string]string
	// Tags filter features by
	Tags []string
	// Path to a directory containing the features to run
	FeaturesPath string
}

// DefaultConfig returns a Config type set to default values
func DefaultConfig() *Config {
	return &Config{
		CreateResources:  true,
		DestroyResources: true,
		LogLevel:         "info",
		Variables:        map[string]string{},
		Tags:             []string{},
		FeaturesPath:     "./",
	}
}

// Creates a configuration from environment variables and flags
// ConfigFromEnv creates a configuration that overrides default values
// using the following flags or environment variables
//
// LogLevel can be set using the LOG_LEVEL=[debug,info,trace] environment
// variable
//
// Tags for the BDD features can be set with either the
// --tag="tag1,tag2" flag or
//SY_TAG=tag1,tag2" environment variable
//
// Variables used when creating resources can be set with either the
// --var="variable=value" flag, (can be set multiple times) or
// SY_ENV_variable="value" environment values
//
// CreateResouces
func ConfigFromEnv() *Config {
	c := DefaultConfig()

	flag.Parse()

	setTagsFromEnv(c)
	setTagsFromFlags(c, flagTags)

	setVariablesFromFlags(c, flagVariables)

	return c
}

func setTagsFromEnv(c *Config) {
	env := os.Getenv("SY_TAG")
	tags := strings.Split(env, ",")
	c.Tags = append(c.Tags, tags...)
}

func setTagsFromFlags(c *Config, flags string) {
	tags := strings.Split(flags, ",")
	c.Tags = append(c.Tags, tags...)
}

func setVariablesFromFlags(c *Config, flags []string) {
	for _, f := range flags {
		parts := strings.Split(f, "=")
		c.Variables[parts[0]] = parts[1]
	}
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	// change this, this is just can example to satisfy the interface
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	v := strings.TrimSpace(value)
	v = strings.TrimPrefix(v, `"`)
	v = strings.TrimSuffix(v, `"`)

	*i = append(*i, v)

	return nil
}
