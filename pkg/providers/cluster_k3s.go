package providers

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/utils"
	"golang.org/x/xerrors"
)

var (
	ErrorClusterInvalidName = errors.New("invalid cluster name")
)

// https://github.com/rancher/k3d/blob/master/cli/commands.go

const k3sBaseImage = "rancher/k3s"

var startTimeout = (120 * time.Second)

func (c *Cluster) createK3s() error {
	c.log.Info("Creating Cluster", "ref", c.config.Name)

	// check the cluster does not already exist
	ids, err := c.client.FindContainerIDs(c.config.Name, c.config.NetworkRef.Name)
	if err != nil {
		return err
	}

	if ids != nil && len(ids) > 0 {
		return ErrorClusterExists
	}

	// create the volume for the cluster
	volID, err := c.client.CreateVolume(c.config.Name)
	if err != nil {
		return err
	}

	// set the image
	image := fmt.Sprintf("%s:%s", k3sBaseImage, c.config.Version)

	// create the server
	// since the server is just a container create the container config and provider
	cc := config.Container{}
	cc.Name = fmt.Sprintf("server.%s", c.config.Name)
	cc.Image = config.Image{Name: image}
	cc.NetworkRef = c.config.NetworkRef
	cc.Privileged = true // k3s must run Privlidged

	// set the volume mount for the images
	cc.Volumes = []config.Volume{
		config.Volume{
			Source:      volID,
			Destination: "/images",
			Type:        "volume",
		},
	}

	// set the environment variables for the K3S_KUBECONFIG_OUTPUT and K3S_CLUSTER_SECRET
	cc.Environment = []config.KV{
		config.KV{Key: "K3S_KUBECONFIG_OUTPUT", Value: "/output/kubeconfig.yaml"},
		config.KV{Key: "K3S_CLUSTER_SECRET", Value: "mysupersecret"}, // This should be random
	}

	// set the API server port to a random number 64000 - 65000
	apiPort := rand.Intn(1000) + 64000
	args := []string{"server", fmt.Sprintf("--https-listen-port=%d", apiPort)}

	// expose the API server port
	cc.Ports = []config.Port{
		config.Port{
			Local:    apiPort,
			Host:     apiPort,
			Protocol: "tcp",
		},
	}

	// disable the installation of traefik
	args = append(args, "--no-deploy=traefik")
	cc.Command = args

	id, err := c.client.CreateContainer(cc)
	if err != nil {
		return err
	}

	// wait for the server to start
	err = c.waitForStart(id)
	if err != nil {
		return err
	}

	// get the Kubernetes config file and drop it in $HOME/.shipyard/config/[clustername]/kubeconfig.yml
	kc, err := c.copyKubeConfig(id)
	if err != nil {
		return xerrors.Errorf("Error copying Kubernetes config: %w", err)
	}

	// create the Docker container version of the Kubeconfig
	// the default KubeConfig has the server location https://localhost:port
	// to use this config inside a docker container we need to use the FQDN for the server
	err = c.createDockerKubeConfig(kc)
	if err != nil {
		return xerrors.Errorf("Error creating Docker Kubernetes config: %w", err)
	}

	// wait for all the default pods like core DNS to start running
	// before progressing
	// we might also need to wait for the api services to become ready
	// this could be done with the folowing command kubectl get apiservice
	err = c.kubeClient.SetConfig(kc)
	if err != nil {
		return err
	}

	err = c.kubeClient.HealthCheckPods([]string{""}, startTimeout)
	if err != nil {
		return xerrors.Errorf("Error while waiting for Kubernetes default pods: %w", err)
	}

	// import the images to the servers container d instance
	// importing images means that k3s does not need to pull from a remote docker hub
	if c.config.Images != nil && len(c.config.Images) > 0 {
		return c.ImportLocalDockerImages(id, c.config.Images)
	}

	return nil
}

func (c *Cluster) destroyK3s() error {
	return nil
}

func (c *Cluster) waitForStart(id string) error {
	start := time.Now()

	for {
		// not running after timeout exceeded? Rollback and delete everything.
		if startTimeout != 0 && time.Now().After(start.Add(startTimeout)) {
			//deleteCluster()
			return errors.New("Cluster creation exceeded specified timeout")
		}

		// scan container logs for a line that tells us that the required services are up and running
		out, err := c.client.ContainerLogs(id, true, true)
		if err != nil {
			out.Close()
			return fmt.Errorf(" Couldn't get docker logs for %s\n%+v", id, err)
		}

		// read from the log and check for Kublet running
		buf := new(bytes.Buffer)
		nRead, _ := buf.ReadFrom(out)
		out.Close()
		output := buf.String()
		if nRead > 0 && strings.Contains(string(output), "Running kubelet") {
			break
		}

		// wait and try again
		time.Sleep(1 * time.Second)
	}

	return nil
}

func (c *Cluster) copyKubeConfig(id string) (string, error) {
	// create destination kubeconfig file paths
	_, destPath, _ := CreateKubeConfigPath(c.config.Name)

	// get kubeconfig file from container and read contents
	err := c.client.CopyFromContainer(id, "/output/kubeconfig.yaml", destPath)
	if err != nil {
		return "", err
	}

	return destPath, nil
}

func (c *Cluster) createDockerKubeConfig(kubeconfig string) error {
	// read the config into a string
	f, err := os.OpenFile(kubeconfig, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	readBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return fmt.Errorf("Couldn't read kubeconfig, %v", err)
	}

	// manipulate the file
	newConfig := strings.Replace(
		string(readBytes),
		"server: https://127.0.0.1",
		fmt.Sprintf("server: https://server.%s", utils.FQDN(c.config.Name, c.config.NetworkRef.Name)),
		-1,
	)

	_, _, dockerPath := CreateKubeConfigPath(c.config.Name)

	kubeconfigfile, err := os.Create(dockerPath)
	if err != nil {
		return fmt.Errorf("Couldn't create kubeconfig file %s\n%+v", dockerPath, err)
	}

	defer kubeconfigfile.Close()
	kubeconfigfile.Write([]byte(newConfig))

	return nil
}

// ImportLocalDockerImages fetches Docker images stored on the local client and imports them into the cluster
func (c *Cluster) ImportLocalDockerImages(clusterID string, images []config.Image) error {
	// pull the images
	// import to volume
	// exec import command

	/*
		vn := volumeName(c.config.Name)
		c.log.Debug("Writing local Docker images to cluster", "ref", c.config.Name, "images", images, "volume", vn)

		imageFile, err := writeLocalDockerImageToVolume(c.client, images, vn, c.log)
		if err != nil {
			return err
		}

		// import the image
		// ctr image import filename
		c.log.Debug("Importing Docker images on cluster", "ref", c.config.Name, "id", clusterID, "image", imageFile)
		err = execCommand(c.client, clusterID, []string{"ctr", "image", "import", imageFile}, c.log.With("parent_ref", c.config.Name))
		if err != nil {
			return err
		}
	*/

	return nil
}

/*



func (c *Cluster) destroyK3s() error {
	c.log.Info("Destroy Cluster", "ref", c.config.Name)

	cc := &config.Container{}
	cc.Name = fmt.Sprintf("server.%s", c.config.Name)
	cc.NetworkRef = c.config.NetworkRef

	cp := NewContainer(cc, c.client, c.log.With("parent_ref", c.config.Name))
	err := cp.Destroy()
	if err != nil {
		return err
	}

	// delete the volume
	return c.deleteVolume()
}

const clusterNameMaxSize int = 35

func validateClusterName(name string) error {
	if err := validateHostname(name); err != nil {
		return err
	}

	if len(name) > clusterNameMaxSize {
		return xerrors.Errorf("cluster name is too long (%d > %d): %w", len(name), clusterNameMaxSize, ErrorClusterInvalidName)
	}

	return nil
}

// ValidateHostname ensures that a cluster name is also a valid host name according to RFC 1123.
func validateHostname(name string) error {
	if len(name) == 0 {
		return xerrors.Errorf("no name provided %w", ErrorClusterInvalidName)
	}

	if name[0] == '-' || name[len(name)-1] == '-' {
		return xerrors.Errorf("hostname [%s] must not start or end with - (dash): %w", name, ErrorClusterInvalidName)
	}

	for _, c := range name {
		switch {
		case '0' <= c && c <= '9':
		case 'a' <= c && c <= 'z':
		case 'A' <= c && c <= 'Z':
		case c == '-':
			break
		default:
			return xerrors.Errorf("hostname [%s] contains characters other than 'Aa-Zz', '0-9' or '-': %w", ErrorClusterInvalidName)

		}
	}

	return nil
}
*/
