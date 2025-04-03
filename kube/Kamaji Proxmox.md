```shell
ssh -i id_testudo ernesto@172.30.232.60

export KUBECONFIG=.kube/config
# Copiato da testudo
kubectl get tcp
kubectl describe tcp tenant-test

# https://kamaji.clastix.io/getting-started/getting-started/

export TENANT_NAMESPACE=default
export TENANT_NAME=tenant-test
export TENANT_PORT=6443

TENANT_ADDR=$(kubectl -n ${TENANT_NAMESPACE} get svc ${TENANT_NAME} -o json | jq -r ."spec.loadBalancerIP")

curl -k https://${TENANT_ADDR}:${TENANT_PORT}/healthz
curl -k https://${TENANT_ADDR}:${TENANT_PORT}/version

kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-admin-kubeconfig -o json \
  | jq -r '.data["admin.conf"]' \
  | base64 --decode \
  > ${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig

kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get svc

kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get ep

kubeadm --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig token create --print-join-command

kubeadm join 172.30.232.66:6443 --token trwpz3.ufe0npfxr5m8ob9s --discovery-token-ca-cert-hash sha256:d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6

sudo apt install conntrack socat
curl -sfL https://goyaki.clastix.io | sudo JOIN_URL=172.30.232.66:6443 JOIN_TOKEN=trwpz3.ufe0npfxr5m8ob9s JOIN_TOKEN_CACERT_HASH=sha256:d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6 JOIN_ASCP=1 KUBERNETES_VERSION=v1.30.5 bash -s join

kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes

curl https://raw.githubusercontent.com/projectcalico/calico/v3.24.1/manifests/calico.yaml -O
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig apply -f calico.yaml

kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes
```

Promox 172.30.232.90/91

```shell
ssh root@172.30.232.90
proxmoxtest

wget https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img -O ubuntu-cloud.img
qemu-img convert -O qcow2 ubuntu-cloud.img ubuntu-cloud.qcow2

qm create $VM_ID --name ubuntu-22.04-template --memory 512 --cores 1 --ciuser ubuntu --cipassword ubuntu
qm importdisk $VM_ID ubuntu-cloud.qcow2 local-lvm
qm set $VM_ID --scsihw virtio-scsi-pci --scsi0 local-lvm:vm-$VM_ID-disk-0
qm set $VM_ID --boot c --bootdisk scsi0
qm set $VM_ID --ide2 local-lvm:cloudinit
qm set $VM_ID --net0 virtio,bridge=vmbr0
qm set $VM_ID --ipconfig0 ip=dhcp
qm set $VM_ID --sshkeys .ssh/id_rsa.pub
qm set $VM_ID --agent enabled=1
qm resize $VM_ID scsi0 16G

qm shutdown $VM_ID

qm start $VM_ID
qm stop $VM_ID

# qm set $VM_ID --ciupgrade 1
# qm set $VM_ID --serial0 socket --vga serial0
# qm resize $VM_ID scsi0 $DISK_SIZE
# qm set $VM_ID --serial0 socket --vga serial0
# qm resize $VM_ID scsi0 $DISK_SIZE
# qm start $VM_ID

# TEMPLATE
qm stop $VM_ID
qm shutdown $VM_ID
qm template $VM_ID

# https://pve.proxmox.com/wiki/Cloud-Init_Support#:~:text=your%20specific%20environment.-,Custom%20Cloud%2DInit%20Configuration,-The%20Cloud%2DInit

export NEW_VM_ID=11001
export NEW_VM_NAME=ubuntu-22.04-test

cat << 'EOF' > user-data.yml
#cloud-config
runcmd:
  - echo "Custom cloud-init setup completed stronzo" >> /root/setup-completed.log
EOF

qm clone $VM_ID $NEW_VM_ID \
  --name $NEW_VM_NAME \
  --full \
  --storage local-lvm \
  --target pve

qm set $NEW_VM_ID --cicustom "vendor=local:snippets/dummy.yaml"

qm guest cmd $NEW_VM_ID network-get-interfaces

qm set $NEW_VM_ID --cicustom "vendo=file:///user-data.yml"
qm start $NEW_VM_ID
qm stop $NEW_VM_ID
qm destroy $NEW_VM_ID
```


--cicustom "user=local:snippets/user-data.yml"



https://bastientraverse.com/en/proxmox-optimized-cloud-init-templates?utm_source=pocket_shared