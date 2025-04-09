


#ritorna i secrets presenti nel namespace default
#devo prendere da qui il token ma prima devo generarlo 
#tenant-test-join-token!!!!!!
curl -s -k \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/json" \
  https://172.30.232.61:6443/api/v1/namespaces/default/secrets

#hash del certificato > CA_CERT
curl -s -k \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/json" \
  https://172.30.232.61:6443/api/v1/namespaces/default/secrets/tenant-test-ca | \
  jq -r '.data."tls.crt"' | base64 -d

# -----BEGIN CERTIFICATE-----
# MIIDBTCCAe2gAwIBAgIIN3XZCx3XrywwDQYJKoZIhvcNAQELBQAwFTETMBEGA1UE
# AxMKa3ViZXJuZXRlczAeFw0yNTAzMzEwNzM1MzFaFw0zNTAzMjkwNzQwMzFaMBUx
# EzARBgNVBAMTCmt1YmVybmV0ZXMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
# AoIBAQC/vtNZ2BXAL+XFAOfqXUrGMYQRDvh4QfyStJJiwb8H7nZ+Vg+UR4U0n5jV
# ZQg7ITpBzo04Au3MPco88OM0+h81LzM3npoQN7KkuqClnY2dBVEqnMVwzMoAKaJ/
# Dz1dtIrBRNO8SyunMGx9bxK1s+DzBaPHir3pn0cIcYbnj6rRBdPsPYZjcnRno1A3
# frBc66vViDv8C9XA+sVoa/1rZMlRBhTQ4UhMw+eii7I10oHg9xAF9u7Buj5k8EFQ
# Mj6+bK/nQS1o9+b/LdNXEC9fNjPpUDBV3zYdrENcLgVUCAdkcavK/rvZwn2XW0GH
# 7DkVfgMXI3sNDt2dyEcFY9UOZ4WpAgMBAAGjWTBXMA4GA1UdDwEB/wQEAwICpDAP
# BgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTDKn3QuRCUE5NxCUZfWP1UGbU3ezAV
# BgNVHREEDjAMggprdWJlcm5ldGVzMA0GCSqGSIb3DQEBCwUAA4IBAQADHyMk5O9s
# DSVb6rSDem6TQIFIMnxm9ILj4WP3E2us8FEQYWaR1EFdTvTpttuDOwsVwABTobQ4
# l2gMWyNJs//VOMRrubbiu6DF6GLP4ZMTyKB+LOOqQkbh1KnrZ9r4XR2j73BG43Vx
# BNJg3Lquzkb9OeICC13xbPzlcoyE2d3V/NF2NFMgRHsg1+aJ5Ih3EKDN8R0eqR+z
# eIcniCB/W6Sc9NTIdTBb20P1NSP9o1rHEHb8ZOEwpIZkS7SPB28d6XqrooOc5pXx
# NxEoaE/1NVsMbwKYWMsojwMd3rfWWD1AzSzKwL2AToZ683kzUa1eGyp7QdBN3C3c
# aWZLW2P/2I3+
# -----END CERTIFICATE-----

CA_CERT_HASH=$(echo $CA_CERT | openssl x509 -pubkey | \
  openssl pkey -pubin -outform DER | \
  openssl dgst -sha256 | sed 's/^.* //')


d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6

NEW_TOKEN=$(openssl rand -hex 3).$(openssl rand -hex 8)

00fe2f.764c8a2371aebfd2

kubeadm join $CONTROL_PLANE_ENDPOINT --token $NEW_TOKEN --discovery-token-ca-cert-hash sha256:$CA_CERT_HASH

kubeadm join 172.30.232.66:6443 --token 00fe2f.764c8a2371aebfd2 --discovery-token-ca-cert-hash sha256:d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6
kubeadm join 172.30.232.66:6443 --token 5a0236.7e4f7f212a5c8936 --discovery-token-ca-cert-hash sha256:d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6


kubectl --kubeconfig=tenant-test.kubeconfig create token admin-user

curl -X POST \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IlNHY1hHQXNWaVdxM2RwQ0Z3TlpuUEU0c3RNUl83cXgwSmhPd0tIeF84bTAifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiXSwiZXhwIjoxNzQ0MjE1ODQ2LCJpYXQiOjE3NDQyMTIyNDYsImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwianRpIjoiZGNjZDBlZDUtZGQxMi00MTM1LWEzMGYtOTBhNWVjYjFhYjJiIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJkZWZhdWx0Iiwic2VydmljZWFjY291bnQiOnsibmFtZSI6ImFkbWluLXVzZXIiLCJ1aWQiOiI2Nzc1NDc5Yi03ZGUyLTRjYjgtYWYzMS02ODE5NWU3NzM5NWIifX0sIm5iZiI6MTc0NDIxMjI0Niwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6YWRtaW4tdXNlciJ9.LujYwDsJbWuUy25cAgpKzheiNnsuHEFaqFLo1rWa6_qW-_KviLUeWaHjl0vexFY61JIXtC01tRu_ILkSvd37yUdQD6avwzwbMza3oGLFhIrSdxzd8FTy3Ihkl-lWygpBSc2qA6m3YncbiMGw9RsSYZSGTGT7XMo62YVE2WrUWLJI1yKC9Hcr3UvxuUgQugDUWXdHg7U2shPCZH_vNWd24i_xF3x3L8sO_G8K0aPX4bxMnWAlkINRezpA30Mcs15-VpprB2TMG6-b69YqGoi_b6gy_95koetGdnaW9j1RmdO8HI2_7N7YilWIgAWRiunZpXj2l7oKOhTQcsqGFWJfvw" \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "TokenRequest",
    "apiVersion": "authentication.k8s.io/v1",
    "spec": {
      "audiences": ["https://kubernetes.default.svc.cluster.local/"],
      "expirationSeconds": 86400
    }
  }' \
  --insecure \
  "https://172.30.232.66:6443/api/v1/namespaces/kube-system/serviceaccounts/bootstrap-signer/token"



  eyJhbGciOiJSUzI1NiIsImtpZCI6IlNHY1hHQXNWaVdxM2RwQ0Z3TlpuUEU0c3RNUl83cXgwSmhPd0tIeF84bTAifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwvIl0sImV4cCI6MTc0NDI5ODY3MCwiaWF0IjoxNzQ0MjEyMjcwLCJpc3MiOiJodHRwczovL2t1YmVybmV0ZXMuZGVmYXVsdC5zdmMuY2x1c3Rlci5sb2NhbCIsImp0aSI6ImI4YjZmZmI4LTY5NTAtNDA1Yy04ZWQxLTA2MDZlYmFlYTk4OSIsImt1YmVybmV0ZXMuaW8iOnsibmFtZXNwYWNlIjoia3ViZS1zeXN0ZW0iLCJzZXJ2aWNlYWNjb3VudCI6eyJuYW1lIjoiYm9vdHN0cmFwLXNpZ25lciIsInVpZCI6ImNmNzQ3ZjFmLTBiNDAtNGRkNC1hZjQzLTE4ZDg5NWJhM2E0ZiJ9fSwibmJmIjoxNzQ0MjEyMjcwLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6a3ViZS1zeXN0ZW06Ym9vdHN0cmFwLXNpZ25lciJ9.CNb8iGN3tkMYH30q99xqBErZxNwbYq228m7yNahtoNEDeLfJdotKoWXJ1BAJa52Sf4mbE3AuzZYeayhu8AlnR6SlojIxHYxubmprRtAjbIx-gNk3Y1qQ7KWZUBVLETny7_BbMeRxvZiZUY7bpfJnDWAmcaPCh7r7zKepphFDylMwZdrkSGCjVMiLDe9GqokutPNqIeduiFJaH6AVEeBxfF5IqUc-x67MiQZuWSOTT0muuqXkdnkMl60Jq9BYNQmCLgOOqyDXvHnib2Y_7yQmGoEVj5zXJWP07o_KSO3UVIEp3gdkcxeY4ODsJcjyuAoZRZshil9NXr63EL8iY-QAtQ


kubectl --kubeconfig=tenant-test.kubeconfig create serviceaccount admin-user
kubectl --kubeconfig=tenant-test.kubeconfig create clusterrolebinding admin-user-binding \
  --clusterrole cluster-admin \
  --serviceaccount default:admin-user




  curl -sfL https://goyaki.clastix.io | sudo JOIN_URL=172.30.232.66:6443 JOIN_TOKEN=a8b6c0.67850592bbc2993a JOIN_TOKEN_CACERT_HASH=sha256:d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6 JOIN_ASCP=1 KUBERNETES_VERSION=v1.30.5 bash -s join



curl -X POST \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IlNHY1hHQXNWaVdxM2RwQ0Z3TlpuUEU0c3RNUl83cXgwSmhPd0tIeF84bTAifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiXSwiZXhwIjoxNzQ0MjE1ODQ2LCJpYXQiOjE3NDQyMTIyNDYsImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwianRpIjoiZGNjZDBlZDUtZGQxMi00MTM1LWEzMGYtOTBhNWVjYjFhYjJiIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJkZWZhdWx0Iiwic2VydmljZWFjY291bnQiOnsibmFtZSI6ImFkbWluLXVzZXIiLCJ1aWQiOiI2Nzc1NDc5Yi03ZGUyLTRjYjgtYWYzMS02ODE5NWU3NzM5NWIifX0sIm5iZiI6MTc0NDIxMjI0Niwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6YWRtaW4tdXNlciJ9.LujYwDsJbWuUy25cAgpKzheiNnsuHEFaqFLo1rWa6_qW-_KviLUeWaHjl0vexFY61JIXtC01tRu_ILkSvd37yUdQD6avwzwbMza3oGLFhIrSdxzd8FTy3Ihkl-lWygpBSc2qA6m3YncbiMGw9RsSYZSGTGT7XMo62YVE2WrUWLJI1yKC9Hcr3UvxuUgQugDUWXdHg7U2shPCZH_vNWd24i_xF3x3L8sO_G8K0aPX4bxMnWAlkINRezpA30Mcs15-VpprB2TMG6-b69YqGoi_b6gy_95koetGdnaW9j1RmdO8HI2_7N7YilWIgAWRiunZpXj2l7oKOhTQcsqGFWJfvw" \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "metadata": {
      "name": "bootstrap-token-a8b6c0",
      "namespace": "kube-system"
    },
    "type": "bootstrap.kubernetes.io/token",
    "data": {
      "auth-extra-groups": "c3lzdGVtOmJvb3RzdHJhcHBlcnM6a3ViZWFkbTpkZWZhdWx0LW5vZGUtdG9rZW4=",
      "token-id": "'"$(echo -n a8b6c0 | base64 -w0)"'",
      "token-secret": "'"$(echo -n 67850592bbc2993a | base64 -w0)"'",
      "expiration": "'"$(date -d "+24 hours" "+%Y-%m-%dT%H:%M:%SZ" | base64 -w0)"'",
      "usage-bootstrap-authentication": "'"$(echo -n true | base64 -w0)"'",
      "usage-bootstrap-signing": "'"$(echo -n true | base64 -w0)"'"
    }
  }' \
  --insecure \
  "https://172.30.232.66:6443/api/v1/namespaces/kube-system/secrets"



kubeadm --kubeconfig=tenant-test.kubeconfig token create --print-join-command


kubeadm join 172.30.232.66:6443 --token a8b6c0.67850592bbc2993a --discovery-token-ca-cert-hash sha256:d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6







{
  "kind": "Secret",
  "apiVersion": "v1",
  "metadata": {
    "name": "bootstrap-token-a8b6c0",
    "namespace": "kube-system",
    "uid": "f5ab11ba-e573-4095-bcb7-d7232eb89a58",
    "resourceVersion": "24101341",
    "creationTimestamp": "2025-04-09T15:58:45Z",
    "managedFields": [
      {
        "manager": "curl",
        "operation": "Update",
        "apiVersion": "v1",
        "time": "2025-04-09T15:58:45Z",
        "fieldsType": "FieldsV1",
        "fieldsV1": {
          "f:data": {
            ".": {},
            "f:auth-extra-groups": {},
            "f:expiration": {},
            "f:token-id": {},
            "f:token-secret": {},
            "f:usage-bootstrap-authentication": {},
            "f:usage-bootstrap-signing": {}
          },
          "f:type": {}
        }
      }
    ]
  },
  "data": {
    "auth-extra-groups": "c3lzdGVtOmJvb3RzdHJhcHBlcnM6a3ViZWFkbTpkZWZhdWx0LW5vZGUtdG9rZW4=",
    "expiration": "MjAyNS0wNC0xMFQxNzo1ODo0NVoK",
    "token-id": "YThiNmMw",
    "token-secret": "Njc4NTA1OTJiYmMyOTkzYQ==",
    "usage-bootstrap-authentication": "dHJ1ZQ==",
    "usage-bootstrap-signing": "dHJ1ZQ=="
  },
  "type": "bootstrap.kubernetes.io/token"
}