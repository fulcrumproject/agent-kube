# INSTALLAZIONE CLUSTER PADRE KAMAJI

## praparazione bootstrap machine
Per l'installazione di kamaji ci avvaliamo di una bootstap machine che deve avere questi software installati:

Kubectl kubeadm helm jq

Bisogna riferirsi alle documentazioni ufficiali per il metodo di installazione

poi scarichiamo i pacchetti di kamaji:
```
git clone https://github.com/clastix/kamaji
```
così scarichiamo i pacchetti di yaki:
```
git clone https://github.com/clastix/yaki
cd yaki/guides
```
preparare gli env:
```
vi setup.env
```
questo un esempio di come impostare il file 
```
# etcd
export ETCD0=172.30.232.56
export ETCD1=172.30.232.57
export ETCD2=172.30.232.58
export ETCDHOSTS=($ETCD0 $ETCD1 $ETCD2)

# control plane
export MASTER0=172.30.232.56
export MASTER1=172.30.232.57
export MASTER2=172.30.232.58
export MASTERS=(${MASTER0} ${MASTER1} ${MASTER2})
export SEED=$MASTER0
export MASTER_VIP=172.30.232.59
export MASTER_PORT=6443
export MASTER_IF=ens18

# workers
#export WORKER0=192.168.31.20
#export WORKER1=192.168.31.21
#export WORKER2=192.168.31.22
#export WORKERS=(${WORKER0} ${WORKER1} ${WORKER2})
#export WORKER_VIP=192.168.31.252
#export WORKER_IF=eth0

# kubernetes
export CLUSTER_NAME=kamaji-ha-sistemi
export CLUSTER_DOMAIN=opiquad.it
export CLUSTER_VERSION=1.30.5
export POD_CIDR=192.168.0.0/16
export SVC_CIDR=10.96.0.0/16
export DNS_SERVICE=10.96.0.10

# infra
export HOSTS=(${MASTER0} ${MASTER1} ${MASTER2}
```
poi possiamo esportare gli env:
```
source setup.env
```
## impostazione accessi ssh dalla bootstrap verso i nodi
per accedere agli altri host dall bootstrap machine dobbiamo creare una chiave ssh (senza password) e distribuirla su tutti i nodi
```
ssh-keygen -t rsa
```
se stiamo usando l'utente root dobbiamo mettere il contenuto del file /root/.ssh/id_rsa.pub (creato sulla bootstrap) all'interno del file /root/.ssh/authorized_keys su ciascun nodo (aggiungendolo al contenuto esistente)

## installazione sui nodi di keepalived (loadbalancer) 
eseguiamo tutto dalla bootstrap

per prima cosa creiamo il file master-keepalived.conf compilandolo con le variabili esportate:
```
cat << EOF | tee master-keepalived.conf
# keepalived global configuration
global_defs {
    default_interface ${MASTER_IF} 
    enable_script_security 
}
vrrp_script apiserver {
    script   "/usr/bin/curl -s -k https://localhost:${MASTER_PORT}/healthz -o /dev/null"
    interval 20
    timeout  5
    rise     1
    fall     1
    user     root
}
vrrp_instance VI_1 {
    state BACKUP
    interface ${MASTER_IF}
    virtual_router_id 100
    priority 10${i}
    advert_int 20
    authentication {
    auth_type PASS
    auth_pass cGFzc3dvcmQ=
    }
    track_script {
    apiserver
    }     
    virtual_ipaddress {
        ${MASTER_VIP} label ${MASTER_IF}:VIP
    }
}
EOF
```
poi, sempre dalla bootstrap eseguiamo l'installazione sui nodi:
```
for i in "${!MASTERS[@]}"; do
MASTER=${MASTERS[$i]}
scp master-keepalived.conf ${USER}@${MASTER}:
ssh ${USER}@${MASTER} -t 'sudo apt update'
ssh ${USER}@${MASTER} -t 'sudo apt install -y keepalived'
ssh ${USER}@${MASTER} -t 'sudo chown -R root:root master-keepalived.conf'
ssh ${USER}@${MASTER} -t 'sudo mv master-keepalived.conf /etc/keepalived/keepalived.conf'
ssh ${USER}@${MASTER} -t 'sudo systemctl restart keepalived'
ssh ${USER}@${MASTER} -t 'sudo systemctl enable keepalived'
done
```


## creazioine dello storage-class per gli etcd
installiamo local-path-provisioner per generare lo storage-class che servirà a creare i persistent volume per gli etcd. facciamo tutto dall bootstrap
```
cd
git clone https://github.com/rancher/local-path-provisioner.git
```
editiamo il file values.yaml per rendere la storage class il default e per definire dove devono essere messi gli etcd dei nodi tenant
```
vi /root/local-path-provisioner/deploy/chart/local-path-provisioner/values.yaml
```
il file viene così:

```
# Default values for local-path-provisioner.

replicaCount: 1
commonLabels: {}

image:
  repository: rancher/local-path-provisioner
  tag: v0.0.30
  pullPolicy: IfNotPresent

helperImage:
  repository: busybox
  tag: latest

defaultSettings:
  registrySecret: ~

privateRegistry:
  registryUrl: ~
  registryUser: ~
  registryPasswd: ~

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

## For creating the StorageClass automatically:
storageClass:
  create: true

  ## Set a provisioner name. If unset, a name will be generated.
  # provisionerName: rancher.io/local-path

  ## Set StorageClass as the default StorageClass
  ## Ignored if storageClass.create is false
  defaultClass: true

  ## The default volume type this storage class creates, can be "local" or "hostPath"
  defaultVolumeType: hostPath

  ## Set a StorageClass name
  ## Ignored if storageClass.create is false
  name: local-path

  ## ReclaimPolicy field of the class, which can be either Delete or Retain
  reclaimPolicy: Delete

  ## volumeBindingMode field controls when volume binding and dynamic provisioning should occur, can be  "Immediate" or "WaitForFirstConsumer"
  volumeBindingMode: WaitForFirstConsumer

  ## Set a path pattern, if unset the default will be used
  # pathPattern: "{{ .PVC.Namespace }}-{{ .PVC.Name }}"

# nodePathMap is the place user can customize where to store the data on each node.
# 1. If one node is not listed on the nodePathMap, and Kubernetes wants to create volume on it, the paths specified in
#    DEFAULT_PATH_FOR_NON_LISTED_NODES will be used for provisioning.
# 2. If one node is listed on the nodePathMap, the specified paths will be used for provisioning.
#     1. If one node is listed but with paths set to [], the provisioner will refuse to provision on this node.
#     2. If more than one path was specified, the path would be chosen randomly when provisioning.
#
# The configuration must obey following rules:
# 1. A path must start with /, a.k.a an absolute path.
# 2. Root directory (/) is prohibited.
# 3. No duplicate paths allowed for one node.
# 4. No duplicate node allowed.
nodePathMap:
  - node: DEFAULT_PATH_FOR_NON_LISTED_NODES
    paths:
      - /var/data/local

# `sharedFileSystemPath` allows the provisioner to use a filesystem that is mounted on all
# nodes at the same time. In this case all access modes are supported: `ReadWriteOnce`,
# `ReadOnlyMany` and `ReadWriteMany` for storage claims. In addition
# `volumeBindingMode: Immediate` can be used in  StorageClass definition.
# Please note that `nodePathMap` and `sharedFileSystemPath` are mutually exclusive.
# If `sharedFileSystemPath` is used, then `nodePathMap` must be set to `[]`.
# sharedFileSystemPath: ""

# `storageClassConfigs` allows the provisioner to manage multiple independent storage classes.
# Each storage class must have a unique name, and contains the same fields as shown above for
# a single storage class setup, EXCEPT for the provisionerName, which is the same for all
# storage classes, and name, which is the key of the map.
# storageClassConfigs: {}
#   my-storage-class:
#     storageClass:
#       create: true
#       defaultClass: false
#       defaultVolumeType: hostPath
#       reclaimPolicy: Delete
#     sharedFileSystemPath: ""
#     ## OR
#     # See above
#     nodePathMap: {}

podAnnotations: {}

podSecurityContext: {}
  # runAsNonRoot: true

securityContext: {}
  # allowPrivilegeEscalation: false
  # seccompProfile:
  #   type: RuntimeDefault
  # capabilities:
  #   drop: ["ALL"]
  # runAsUser: 65534
  # runAsGroup: 65534
  # readOnlyRootFilesystem: true

resources: {}
  # We usually recommend not to specify default resources and to leave this as a conscious
  # choice for the user. This also increases chances charts run on environments with little
  # resources, such as Minikube. If you do want to specify resources, uncomment the following
  # lines, adjust them as necessary, and remove the curly braces after 'resources:'.
  # limits:
  #   cpu: 100m
  #   memory: 128Mi
  # requests:
  #   cpu: 100m
  #   memory: 128Mi

helperPod:
  resources: {}
    # limits:
    #   cpu: 100m
    #   memory: 128Mi
    # requests:
    #   cpu: 100m
    #   memory: 128Mi

rbac:
  # Specifies whether RBAC resources should be created
  create: true

serviceAccount:
  # Specifies whether a ServiceAccount should be created
  create: true
  # The name of the ServiceAccount to use.
  # If not set and create is true, a name is generated using the fullname template
  name:

nodeSelector: {}

tolerations: []

affinity: {}

configmap:
  # specify the config map name
  name: local-path-config
  # specify the custom script for setup and teardown
  setup: |-
    #!/bin/sh
    set -eu
    mkdir -m 0777 -p "$VOL_DIR"
  teardown: |-
    #!/bin/sh
    set -eu
    rm -rf "$VOL_DIR"
  helperPod:
    # Allows to run the helper pod in another namespace. Uses release namespace by default.
    namespaceOverride: ""
    name: "helper-pod"
    annotations: {}

# Number of provisioner worker threads to call provision/delete simultaneously.
# workerThreads: 4

# Number of retries of failed volume provisioning. 0 means retry indefinitely.
# provisioningRetryCount: 15

# Number of retries of failed volume deletion. 0 means retry indefinitely.
# deletionRetryCount: 15
```

## preparazione dei nodi su cui installare il cluster padre:
Tutti i nodi devono avere hostname unico ed ip (privato o pubblico) dedicato

Tutti i nodi devono avere swap disabilitata

Nell'installzione della vm, oltre al normale disco, servono 2 ulteriori dischi per mettere gli etcd

uno da 12gb montato su /var/lib/etcd/ (per l'etcd del cluster padre)

l'altro più grosso (serviranno 8 gb per ogni tenant) per tenere gli etcd di tutti i tenant. montato su /var/data/local/

nell'installazione di test stiamo usando dischi da 200gb per questa partizione 

Eseguire questi comandi su tutti i nodi:
```
apt update && apt upgrade -y
apt install conntrack socat wget -y
```
verificare anche la presenza dei seguenti pacchetti, nel caso installarli:
ip, iptables, modprobe, sysctl, systemctl, nsenter, ebtables, ethtool

## installazione primo nodo ed inizializzazione cluster

Iniziamo con creare il file kubeadm-config.yaml, compilato con i nostri valori esportati:
```
cat > kubeadm-config.yaml <<EOF  
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
bootstrapTokens:
- groups:
  - system:bootstrappers:kubeadm:default-node-token
  token:
  ttl: 48h0m0s
  usages:
  - signing
  - authentication
localAPIEndpoint:
  advertiseAddress: "0.0.0.0"
  bindPort: ${MASTER_PORT}
nodeRegistration:
  criSocket: unix:///run/containerd/containerd.sock
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
clusterName: ${CLUSTER_NAME}
certificatesDir: /etc/kubernetes/pki
imageRepository: registry.k8s.io
networking:
  dnsDomain: cluster.local
  podSubnet: ${POD_CIDR}
  serviceSubnet: ${SVC_CIDR}
dns:
  imageRepository: registry.k8s.io/coredns
  imageTag: v1.9.3
controlPlaneEndpoint: "${MASTER_VIP}:${MASTER_PORT}"
kubernetesVersion: "${CLUSTER_VERSION}"
etcd:
  local:
    dataDir: /var/lib/etcd/data
apiServer:
  certSANs:
  - localhost
  - ${MASTER_VIP}
  - ${CLUSTER_NAME}.${CLUSTER_DOMAIN}
scheduler:
  extraArgs:
    bind-address: "0.0.0.0" # required to expose metrics
controllerManager:
  extraArgs:
    bind-address: "0.0.0.0" # required to expose metrics
---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
metricsBindAddress: "0.0.0.0" # required to expose metrics
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd  # tells kubelet about cgroup driver to use (required by containerd)
EOF
```
dopodiché mettimo il file nel primo dodo (nodo SEED)
```
scp kubeadm-config.yaml ${USER}@${SEED}:
```
poi possiamo fare l'installzione con yaki e l'inizializzazione del cluster (agendo sempre dalla bootstrap)
```
ssh ${USER}@${SEED} -t 'sudo env KUBEADM_CONFIG=kubeadm-config.yaml bash -s'
ssh ${USER}@${SEED} -t 'curl -sfL https://goyaki.clastix.io | sudo bash -s init'
```

una volta inizializzato il cluster dobbiamo prendere nota dei parametri necessari all'affiliazione degli altri nodi, qui sotto un esempio:
```
 JOIN_URL=172.30.232.56:6443
 JOIN_TOKEN=g244gi.ckpyu0c0pmimfpro
 JOIN_TOKEN_CERT_KEY=8e4e922a299b745eea088e0e8a133925b82d0e205f7a3dc4f84dcfa1ea38e42a
 JOIN_TOKEN_CACERT_HASH=sha256:b7f08b3d3037d9baa88cc4c0ccfc9bdc5d2a8e0dfe8b9af114b0ffda67c88709
```
creare il file .kubeconfig per potervi accedere dalla bootstrap machine:
```
  ssh ${USER}@${SEED} -t 'sudo cp -i /etc/kubernetes/admin.conf .'
  ssh ${USER}@${SEED} -t 'sudo chown $(id -u):$(id -g) admin.conf'
  mkdir -p $HOME/.kube
  scp ${USER}@${SEED}:admin.conf $HOME/.kube/${CLUSTER_NAME}.kubeconfig
```
esportare il file kubeconfig e verificare lo stato del cluster:
```
export KUBECONFIG=$HOME/.kube/${CLUSTER_NAME}.kubeconfig
kubectl cluster-info
```

Ed ora la nostra cli eseguirà i comandi specificati sul nostro cluster appena creato


## installazione ed affiliazioni degli altri nodi control-plane

Per fare l'installazione sugli altri nodi ed il join degli stessi al cluster col ruolo di control-plane dobbiamo compilare il seguente comando:
```
curl -sfL https://goyaki.clastix.io | sudo JOIN_URL=<control-plane-endpoint>:<port> JOIN_TOKEN=<token> JOIN_TOKEN_CERT_KEY=<key> JOIN_TOKEN_CACERT_HASH=sha256:<hash> JOIN_ASCP=1 KUBERNETES_VERSION=v1.30.5 bash -s join
```
Che, compilato con i dati che abbiamo ricavato dall'installazione del nodo master, per noi diventerà una cosa di questo tipo:
```
curl -sfL https://goyaki.clastix.io | sudo JOIN_URL=172.30.232.61:6443 JOIN_TOKEN=osxp4o.oo5mk6kdj5h30rcf JOIN_TOKEN_CERT_KEY=75c4a3166a97b59df05bde82e11389f90a88c69a506e5f286439b6244dabfc58 JOIN_TOKEN_CACERT_HASH=sha256:5fe8a04952e769b2f5e6285428d3405b9f723a8b66ba3e270be09d2bf2bfaa0a JOIN_ASCP=1 KUBERNETES_VERSION=v1.30.5 bash -s join
```
Lo eseguiamo sugli altri nodi del cluster.

Ora i nostri nodi sono affiliati al cluster:
```
root@kam-test-sviluppo-bootstrap:~# k get nodes
NAME                   STATUS     ROLES           AGE    VERSION
kam-test-sviluppo-n1   NotReady   control-plane   22m    v1.30.5
kam-test-sviluppo-n2   NotReady   control-plane   115s   v1.30.5
kam-test-sviluppo-n3   NotReady   control-plane   26s    v1.30.5
```
Non sono ancora pronti perché manca la CNI a gestire il networking tra i container

## installazione CNI

Dalla bootstrap scarichiamo il file di una CNI (calcio nel nostro caso) e lo eseguiamo in Kubernetes:
```
wget https://docs.projectcalico.org/manifests/calico.yaml
kubectl create -f calico.yaml
```
Con questo comando otteniamo monitorata l'installazione per vedere quando finisce:
```
k get po -A -o wide -w
```
Tutti i pod devono essere in running

Ora tutti i nodi sono pronti:
```
root@kam-test-sviluppo-bootstrap:~# k get nodes
NAME                   STATUS   ROLES           AGE     VERSION
kam-test-sviluppo-n1   Ready    control-plane   28m     v1.30.5
kam-test-sviluppo-n2   Ready    control-plane   7m15s   v1.30.5
kam-test-sviluppo-n3   Ready    control-plane   5m46s   v1.30.5
```

## installazione prerequisiti kubernetes sul cluster appena creato
dalla bootstrap machine facciamo le installazioni kubernetes 

### metallb
Installiamo metallb come load balancer per l'esposizione dei tenant:
```
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.8/config/manifests/metallb-native.yaml
```
Poi dobbiamo configurarlo. Creiamo un file yaml con questi contenuti (editando il pool di indirizzi ad hoc):
```
vi kam-test-sviluppo-matallb-manifest.yaml
```
```
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  namespace: metallb-system
  name: my-ip-pool
spec:
  addresses:
  - 172.30.232.230-172.30.232.240

---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  namespace: metallb-system
  name: my-l2-advertisement
spec: {}
```
Prima di applicare lo yaml dobbiamo rimuovere una label dai nodi:

```
kubectl label nodes --all node.kubernetes.io/exclude-from-external-load-balancers-
```
ed un taint:
```
k taint node kamaji-n1 kamaji-n2 kamaji-n3 node-role.kubernetes.io/control-plane-
```


### cert manager
Installiamo certi-manager sul cluster dalla bootstrap:
```
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.11.0 \
  --set installCRDs=true
```
## Finalizziamo l'installazione di local-path-storage con la sua helm chart 

prima proviamo un dry run per vedere se da problemi
```
cd /root/local-path-provisioner/deploy/chart/local-path-provisioner/
helm install localstorage -n local-path-storage --create-namespace . --dry-run
helm install localstorage -n local-path-storage --create-namespace .
```


## installazione kamaji
```
helm repo add clastix https://clastix.github.io/charts
helm repo update
helm install kamaji clastix/kamaji -n kamaji-system --create-namespace
```

verificare che tutti i pod siano in running

```
k get po -A
```
