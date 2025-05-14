#cloud-config
hostname: {{.Hostname}}
manage_etc_hosts: true
fqdn: {{.FQDN}}
user: {{.Username}}
password: {{.Password}}
ssh_authorized_keys:
{{- range .SSHKeys}}
  - {{.}}
{{- end}}
chpasswd:
  expire: {{.ExpirePassword}}
users:
  - default
package_upgrade: {{.PackageUpgrade}}
runcmd:
  - curl -sfL https://goyaki.clastix.io | sudo JOIN_URL={{.JoinURL}} JOIN_TOKEN={{.JoinToken}} JOIN_TOKEN_CACERT_HASH={{.CACertHash}} JOIN_ASCP=1 KUBERNETES_VERSION={{.KubeVersion}} bash -s join