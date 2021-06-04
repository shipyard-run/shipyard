package parser

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
)

func setupTestConfig(t *testing.T, contents string) (*config.Config, string, func()) {
	e, _ := setup(t)
	dir, cleanup := createTestFiles(t)
	createNamedFile(t, dir, "*.hcl", contents)

	c := config.New()
	err := e.ParseFolder(dir, c, false, "", false, []string{}, nil, "")
	assert.NoError(t, err)

	err = e.ParseReferences(c)
	assert.NoError(t, err)

	return c, dir, cleanup
}

// createsTestFiles creates a temporary directory and
// stores temp files into it
// returns directory containing files
// cleanup function
// usage:
// d, cleanup := createTestFiles(t, `cluster "abc" {}`, `docs "bcdf" {}`)
// defer cleanup()
func createTestFiles(t *testing.T, contents ...string) (string, func()) {
	dir := createTempDirectory(t)

	for _, x := range contents {
		createTestFile(t, dir, x)
	}

	return dir, func() {
		removeTestFiles(t, dir)
	}
}

// create a temporary directory
func createTempDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Unable to create temporary directory: %s", err)
	}

	return dir
}

// creates a temporary file for testing
func createTestFile(t *testing.T, dir, contents string) string {
	return createNamedFile(t, dir, "*.hcl", contents)
}

func createNamedFile(t *testing.T, dir, name, contents string) string {
	f, err := ioutil.TempFile(dir, name)
	if err != nil {
		t.Fatalf("Error creating temp file %s", err)
	}
	defer f.Close()

	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("Error writing temp file contents: %s", err)
	}

	return f.Name()
}

// remove test files cleans up any temporary files created
// with createTestFile
func removeTestFiles(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("Unable to remove temporary files %s", err)
	}
}
