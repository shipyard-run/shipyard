k8s_cluster "k3s" {
  driver  = "k3s" // defaultA
  version = "v1.18.16"

  nodes = 1 // default

  network {
    name = "network.cloud"
  }

  image {
    name = "shipyardrun/connector:v0.0.10"
  }
}
