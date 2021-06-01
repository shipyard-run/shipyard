package clients

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/hashicorp/go-hclog"
	"github.com/shipyard-run/shipyard/pkg/clients/mocks"
	clients "github.com/shipyard-run/shipyard/pkg/clients/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var testCopyLocalImages = []string{"consul:1.6.1"}
var testCopyLocalVolume = "images"

// Create happy path mocks
func testCreateCopyLocalMocks() *mocks.MockDocker {
	mk := &mocks.MockDocker{}

	mk.On("ImageSave", mock.Anything, mock.Anything).Return(
		ioutil.NopCloser(bytes.NewBufferString("test")),
		nil,
	)

	mk.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(container.ContainerCreateCreatedBody{ID: "myid"}, nil)

	mk.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mk.On(
		"CopyToContainer",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	mk.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mk.On("ImageList", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	mk.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).
		Return(ioutil.NopCloser(strings.NewReader("hello world")), nil)

	mk.On("ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything).
		Return(types.IDResponse{ID: "abc"}, nil)

	mk.On("ContainerExecAttach", mock.Anything, "abc", mock.Anything).Return(
		types.HijackedResponse{
			Conn: &net.TCPConn{},
			Reader: bufio.NewReader(
				bytes.NewReader([]byte("log output")),
			),
		},
		nil,
	)

	mk.On("ContainerExecStart", mock.Anything, "abc", mock.Anything).Return(nil)

	mk.On("ContainerExecInspect", mock.Anything, "abc", mock.Anything).
		Return(types.ContainerExecInspect{Running: false, ExitCode: 0}, nil)

	mk.On("VolumeList", mock.Anything, mock.Anything).
		Return(volume.VolumeListOKBody{Volumes: []*types.Volume{&types.Volume{}}})

	return mk
}

func TestCopyLocalDoesNothingWhenCached(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)

	args := types.ExecConfig{
		Cmd: []string{
			"find",
			"/cache/images/" +
				base64.RawURLEncoding.EncodeToString([]byte(testCopyLocalImages[0])),
		},
		WorkingDir:   "/",
		AttachStdout: true,
		AttachStderr: true,
	}

	mk.AssertCalled(t, "ContainerExecCreate", mock.Anything, "myid", args)
	mk.AssertNotCalled(t, "ImageSave")
}

func TestCopyLocalDoesNotChecksVolumeCacheWhenGlobalForce(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)

	// should have been called once to create the directory
	mk.AssertNumberOfCalls(t, "ContainerExecCreate", 1)
}

func TestCopyLocalDoesNotChecksVolumeCacheWhenLocalForce(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(false) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, true)
	assert.NoError(t, err)

	// should have been called once to create the directory
	mk.AssertNumberOfCalls(t, "ContainerExecCreate", 1)
}

func TestCopyLocalSavesImages(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)
	mk.AssertCalled(t, "ImageSave", mock.Anything, testCopyLocalImages)
}

func TestCopyLocalSavesImageFailReturnsError(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	removeOn(&mk.Mock, "ImageSave")
	mk.On("ImageSave", mock.Anything, mock.Anything).Return(
		nil,
		fmt.Errorf("blah"),
	)
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.Error(t, err)
}

func TestCopyLocalCreatesTempContainer(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)

	// ensure it mounts the volume
	params := getCalls(&mk.Mock, "ContainerCreate")[0].Arguments
	hc := params[2].(*container.HostConfig)
	cfg := params[1].(*container.Config)

	// test name and command
	assert.Contains(t, cfg.Hostname, "import")
	assert.Equal(t, "tail", cfg.Cmd[0])

	// test mounts volume
	assert.Len(t, hc.Mounts, 1)
	assert.Equal(t, testCopyLocalVolume, hc.Mounts[0].Source)
	assert.Equal(t, "/cache", hc.Mounts[0].Target)
	assert.Equal(t, mount.TypeVolume, hc.Mounts[0].Type)
}

func TestCopyLocalTempContainerFailsReturnError(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	removeOn(&mk.Mock, "ContainerCreate")
	mk.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(container.ContainerCreateCreatedBody{}, fmt.Errorf("boom"))
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.Error(t, err)
}

func TestCopyLocalPullsImportImage(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)
	mk.AssertCalled(t, "ImagePull", mock.Anything, makeImageCanonical("alpine:latest"), mock.Anything)
}
func TestCopyLocalCreatesDestinationDirectory(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)

	mk.AssertCalled(t, "ContainerExecCreate", mock.Anything, "myid", mock.Anything)

	params := getCalls(&mk.Mock, "ContainerExecCreate")[0].Arguments[2].(types.ExecConfig)
	assert.Equal(t, []string{"mkdir", "-p", "/cache/images"}, params.Cmd)
}

func TestCopyLocalCopiesArchive(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)
	mk.AssertCalled(t, "CopyToContainer", mock.Anything, mock.Anything, "/cache/images", mock.Anything, mock.Anything)
}

func TestCopyLocalCopiesArchiveFailReturnsError(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)
	removeOn(&mk.Mock, "CopyToContainer")
	mk.On("CopyToContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("boom"))

	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.Error(t, err)
}

func TestCopyLocalRemovesTempContainer(t *testing.T) {
	mk := testCreateCopyLocalMocks()
	mic := &clients.ImageLog{}
	mic.On("Log", mock.Anything, mock.Anything).Return(nil)

	dt := NewDockerTasks(mk, mic, hclog.NewNullLogger())
	dt.SetForcePull(true) // set force pull to avoid execute command block

	_, err := dt.CopyLocalDockerImagesToVolume(testCopyLocalImages, testCopyLocalVolume, false)
	assert.NoError(t, err)
	mk.AssertCalled(t, "ContainerRemove", mock.Anything, mock.Anything, mock.Anything)
}
