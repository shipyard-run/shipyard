variable "version" {
  default = "consul:1.6.1"
}

network "onprem" {
  subnet = "10.6.0.0/16"
}

container "consul" {
  image   {
    name = var.version
  }

  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  network   {
    name = "network.onprem"
    ip_address = "10.6.0.200"
  }
  
  port_range {
    range       = "8500-8502"
    enable_host = true
  }

  resources {
    # Max CPU to consume, 1024 is one core, default unlimited
    cpu = 2048
    # Pin container to specified CPU cores, default all cores
    cpu_pin = [1]
    # max memory in MB to consume, default unlimited
    memory = 1024
  }
}
