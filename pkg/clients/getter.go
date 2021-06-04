package clients

import (
	"context"
	"os"

	"github.com/hashicorp/go-getter"
	"golang.org/x/xerrors"
)

// Getter is an interface which defines interations for
// downloading remote folders
type Getter interface {
	Get(uri, dst string) error
	SetForce(force bool)
}

// GetterImpl is a concrete implementation of the Getter interface
type GetterImpl struct {
	//
	force bool
	get   func(uri, dst, pwd string) error
}

// NewGetter creates a new Getter
func NewGetter(force bool) *GetterImpl {
	gi := &GetterImpl{
		force,
		func(uri, dst, pwd string) error {
			// if the argument is a url fetch it first
			c := &getter.Client{
				Ctx:     context.Background(),
				Src:     uri,
				Dst:     dst,
				Pwd:     pwd,
				Mode:    getter.ClientModeAny,
				Options: []getter.ClientOption{},
			}

			return c.Get()
		},
	}

	return gi
}

// SetForce sets the force flag causing all downloads to overwrite the destination
func (g *GetterImpl) SetForce(force bool) {
	g.force = force
}

// Get attempts to retrieve a folder
// from a remote location and stores it at the destination.
//
// If force was set to true when creating a Getter then
// the destination folder will automatically be overwritten.
//
// Returns error on failure
func (g *GetterImpl) Get(uri, dst string) error {
	// check to see if a folder exists at the destination and exit if force is not
	// equal to true
	_, err := os.Stat(dst)
	if err == nil {
		// we already have files at the destination do we want to overwrite?
		if g.force == false {
			return nil
		}

		err := os.RemoveAll(dst)
		if err != nil {
			return xerrors.Errorf("Destination folder exists, unable to delete: %w", err)
		}
	}

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = g.get(uri, dst, pwd)
	if err != nil {
		return xerrors.Errorf("Unable to fetch files from %s dest: %s working: %s: %w", uri, dst, pwd, err)
	}

	return nil
}
