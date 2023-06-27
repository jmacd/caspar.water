# Run nomad-pack run influxdb -f ./influxdb.hcl

job_name = "influxdb"
image_name = "influxdb"
image_tag = "2.7.1-alpine"
datacenters = ["dc1"]
config_volume_name = "influxconfig"
config_volume_type = "host"
data_volume_name = "influxdata"
data_volume_type = "host"
docker_influxdb_env_vars = {
  "INFLUXD_CONFIG_PATH" = "/etc/influxdb2/config.yaml"
}
