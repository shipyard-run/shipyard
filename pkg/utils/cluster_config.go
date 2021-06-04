package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

// ClusterConfig defines a config file which is used to store Nomad cluster
// connection info
type ClusterConfig struct {
	// Location of the Cluster
	LocalAddress  string `json:"local_address"`
	RemoteAddress string `json:"remote_address"`

	// Port the API Server is running on
	APIPort int `json:"api_port"`

	// Port the API Server is running on reachable from
	// a remote network
	RemoteAPIPort int `json:"remote_api_port"`

	// Port where the connector is running
	ConnectorPort int `json:"connector_port"`

	// Number of nodes in the cluster
	NodeCount int `json:"node_count"`

	// Does the API use SSL?
	SSL bool `json:"ssl"`

	context Context
}

// Context is a type which stores the context for the cluster
type Context string

// LocalContext defines a constant for the local context
const LocalContext Context = "local"

// RemoteContext defines a constant for the remote context
const RemoteContext Context = "remote"

// Load the config from a file
func (n *ClusterConfig) Load(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewDecoder(f).Decode(n)
}

// Save the config to a file
func (n *ClusterConfig) Save(file string) error {
	// if the file exists delete
	fs, err := os.Stat(file)
	if err != nil && fs != nil {
		err := os.Remove(file)
		if err != nil {
			return err
		}
	}

	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(n)
}

// APIAddress returns the FQDN for the API server
func (n *ClusterConfig) APIAddress(context Context) string {
	protocol := "http"
	if n.SSL {
		protocol = "https"
	}

	if context == LocalContext {
		return fmt.Sprintf("%s://%s:%d", protocol, n.LocalAddress, n.APIPort)
	}

	return fmt.Sprintf("%s://%s:%d", protocol, n.RemoteAddress, n.RemoteAPIPort)
}

// ConnectorAddress returns the FQDN for the gRPC endpoing of the Connector
func (n *ClusterConfig) ConnectorAddress(context Context) string {
	if context == LocalContext {
		return fmt.Sprintf("%s:%d", n.LocalAddress, n.ConnectorPort)
	}

	return fmt.Sprintf("%s:%d", n.RemoteAddress, n.ConnectorPort)
}
