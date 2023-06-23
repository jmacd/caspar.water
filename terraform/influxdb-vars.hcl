# Run nomad-pack run influxdb -f ./influxdb.hcl

job_name = "influxdb-dev"
datacenters = ["dc1"]
config_volume_name = "influxconfig"
config_volume_type = "host"
data_volume_name = "influxdata"
data_volume_type = "host"

