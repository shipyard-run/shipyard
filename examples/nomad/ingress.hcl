container_ingress "consul-http" {
  target  = "container.consul"

  port {
    local  = 8500
    remote = 8500
    host   = 18500
  }

  network  {
    name = "network.cloud"
  }
}

nomad_ingress "fake-service-1" {
  cluster  = "nomad_cluster.dev"
  job = "example_1"
  group = "fake_service"
  task = "fake_service"

  port {
    local  = 19090
    remote = "http"
    host   = 19090
  }

  network  {
    name = "network.cloud"
  }
}

nomad_ingress "fake-service-2" {
  cluster  = "nomad_cluster.dev"
  job = "example_2"
  group = "fake_service"
  task = "fake_service"

  port {
    local  = 19090
    remote = "http"
    host   = 19091
  }

  network  {
    name = "network.cloud"
  }
}
