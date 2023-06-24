data_dir  = "/opt/nomadclient/ports"

server {
  enabled = false
}

ports {
  http = 4649
}

client {
  enabled = true
  servers = ["0.0.0.0:4647"]

  host_volume "influxconfig" {
    path = "/etc/caspar.d/influxdb"
    read_only = false
  }
  host_volume "influxdata" {
    path      = "/opt/influxdb"
    read_only = false
  }
}
