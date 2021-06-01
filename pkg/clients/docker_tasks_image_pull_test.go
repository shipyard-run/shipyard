package clients

import (
	"encoding/base64"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/hashicorp/go-hclog"
	"github.com/shipyard-run/shipyard/pkg/clients/mocks"
	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupImagePullMocks() (*mocks.MockDocker, *mocks.ImageLog) {
	md := &mocks.MockDocker{}
	md.On("ImageList", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	md.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(
		ioutil.NopCloser(strings.NewReader("hello world")),
		nil,
	)

	mic := &mocks.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	mic.On("Read", mock.Anything, mock.Anything).Return([]string{}, nil)

	return md, mic
}

func createImagePullConfig() (config.Image, *mocks.MockDocker, *mocks.ImageLog) {
	ic := config.Image{
		Name: "consul:1.6.1",
	}

	mk, mic := setupImagePullMocks()
	return ic, mk, mic
}

func setupImagePull(t *testing.T, cc config.Image, md *mocks.MockDocker, mic *mocks.ImageLog, force bool) {
	p := NewDockerTasks(md, mic, hclog.NewNullLogger())

	// create the container
	err := p.PullImage(cc, force)
	assert.NoError(t, err)

	return
}

func TestPullImageWhenNOTCached(t *testing.T) {
	cc, md, mic := createImagePullConfig()
	setupImagePull(t, cc, md, mic, false)

	// test calls list image with a canonical image reference
	args := filters.NewArgs(filters.KeyValuePair{Key: "reference", Value: cc.Name})
	md.AssertCalled(t, "ImageList", mock.Anything, types.ImageListOptions{Filters: args})

	// test pulls image replacing the short name with the canonical registry name
	md.AssertCalled(t, "ImagePull", mock.Anything, makeImageCanonical(cc.Name), types.ImagePullOptions{})

	// test adds to the cache log
	mic.AssertCalled(t, "Log", mock.Anything, mock.Anything)
}

func TestPullImageWithCredentialsWhenNOTCached(t *testing.T) {
	cc, md, mic := createImagePullConfig()
	cc.Username = "nicjackson"
	cc.Password = "S3cur1t11"

	setupImagePull(t, cc, md, mic, false)

	// test calls list image with a canonical image reference
	args := filters.NewArgs(filters.KeyValuePair{Key: "reference", Value: cc.Name})
	md.AssertCalled(t, "ImageList", mock.Anything, types.ImageListOptions{Filters: args})

	// test pulls image replacing the short name with the canonical registry name
	// adding credentials to image pull
	ipo := types.ImagePullOptions{RegistryAuth: createRegistryAuth(cc.Username, cc.Password)}
	md.AssertCalled(t, "ImagePull", mock.Anything, makeImageCanonical(cc.Name), ipo)

}

func TestPullImageWithValidCredentials(t *testing.T) {
	cc, md, mic := createImagePullConfig()
	cc.Username = "nicjackson"
	cc.Password = "S3cur1t11"

	setupImagePull(t, cc, md, mic, false)

	ipo := getCalls(&md.Mock, "ImagePull")[0].Arguments[2].(types.ImagePullOptions)

	d, err := base64.RawURLEncoding.DecodeString(ipo.RegistryAuth)
	assert.NoError(t, err)
	assert.Equal(t, `{"Username": "nicjackson", "Password": "S3cur1t11"}`, string(d))
}

// validate the registry auth is in the correct format
func TestPullImageNothingWhenCached(t *testing.T) {
	cc, md, mic := createImagePullConfig()

	// remove the default image list which returns 0 cached images
	removeOn(&md.Mock, "ImageList")
	md.On("ImageList", mock.Anything, mock.Anything, mock.Anything).Return([]types.ImageSummary{types.ImageSummary{}}, nil)

	setupImagePull(t, cc, md, mic, false)

	md.AssertNotCalled(t, "ImagePull", mock.Anything, mock.Anything, mock.Anything)
	mic.AssertNotCalled(t, "Log", mock.Anything, mock.Anything)
}

func TestPullImageAlwaysWhenForce(t *testing.T) {
	cc, md, mic := createImagePullConfig()

	setupImagePull(t, cc, md, mic, true)

	md.AssertNotCalled(t, "ImageList", mock.Anything, mock.Anything)
	md.AssertCalled(t, "ImagePull", mock.Anything, mock.Anything, mock.Anything)
	mic.AssertCalled(t, "Log", mock.Anything, mock.Anything)
}
