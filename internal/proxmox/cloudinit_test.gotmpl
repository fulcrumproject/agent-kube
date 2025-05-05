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
  - echo "JOIN_URL={{.JoinURL}} JOIN_TOKEN={{.JoinToken}} JOIN_TOKEN_CACERT_HASH={{.CACertHash}} KUBERNETES_VERSION={{.KubeVersion}}" > /tmp/cloudinit-fake.txt
