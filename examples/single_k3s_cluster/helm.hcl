helm "consul" {
  cluster = "k8s_cluster.k3s"
  chart = "./helm/consul-helm-0.22.0"
  values = "./helm/consul-values.yaml"
  
  health_check {
    timeout = "60s"
    pods = ["release=consul"]
  }
}

helm "vault" {
  cluster = "k8s_cluster.k3s"
  chart = "github.com/hashicorp/vault-helm?ref=v0.11.0"

  values_string = {
    "server.dataStorage.size" = "128Mb",
    "server.dev.enabled" = "true",
    "server.standalone.enabled" = "true",
    "server.authDelegator.enabled" = "true"
  }

  health_check {
    timeout = "120s"
    pods = ["app.kubernetes.io/name=vault"]
  } 
}
