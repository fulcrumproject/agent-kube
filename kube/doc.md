# Documentazione API Kamaji e Proxmox

## Indice
- [1. Gestione Tenant Control Plane](#1-gestione-tenant-control-plane)
  - [1.1 Creazione TCP](#11-creazione-tcp)
  - [1.2 Gestione TCP](#12-gestione-tcp)
  - [1.3 Monitoraggio Risorse](#13-monitoraggio-risorse)
- [2. Gestione VM Proxmox](#2-gestione-vm-proxmox)
  - [2.1 Autenticazione](#21-autenticazione)
  - [2.2 Operazioni sulle VM](#22-operazioni-sulle-vm)
  - [2.3 Monitoraggio VM](#23-monitoraggio-vm)

## 1. Gestione Tenant Control Plane

### 1.1 Creazione TCP

#### Configurazione YAML
Il seguente YAML definisce la configurazione base per un Tenant Control Plane:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant-test
  labels:
    tenant.clastix.io: tenant-test
spec:
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  kubernetes:
    version: "v1.30.0"
    kubelet:
      cgroupfs: systemd
  networkProfile:
    port: 6443
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity:
      server:
        port: 8132
```

#### Creazione via API
Esistono due modalità per creare un TCP:

(! capire come creare il token da utilizzare)

1. **Chiamata diretta con token**:
```bash
curl -X POST https://172.30.232.60/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>" \
  -H "Content-Type: application/yaml" \
  --data-binary @tenant-control-plane.yaml 
```

2. **Utilizzo del proxy kubectl** (metodo consigliato per sviluppo locale):
```bash
# Avvio proxy
kubectl proxy

# Creazione TCP
curl -X POST http://127.0.0.1:8001/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes \
  -H "Content-Type: application/yaml" \
  --data-binary @tenant-control-plane.yaml
```

### 1.2 Gestione TCP

#### Eliminazione TCP
```bash
curl -X DELETE http://127.0.0.1:8001/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes/tenant-test
```

#### Aggiornamento TCP

1. **Modifica parametro singolo** (esempio: modifica repliche):
```bash
curl -X PATCH http://127.0.0.1:8001/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes/tenant-test \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"spec": {"controlPlane": {"deployment": {"replicas": 3}}}}'
```

2. **Sostituzione configurazione completa**:
```bash
curl -X PUT http://127.0.0.1:8001/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes/tenant-test \
  -H "Content-Type: application/json" \
  --data-binary @tenantcontrolplane.json
```

### 1.3 Monitoraggio Risorse

#### Query Informazioni Cluster

- **Lista TCP**:
```bash
curl -s http://127.0.0.1:8001/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes
```

- **Dettagli TCP Specifico**:
```bash
curl -s http://127.0.0.1:8001/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes/tenant-test
```

- **Deployments**:
```bash
curl -s http://127.0.0.1:8001/apis/apps/v1/namespaces/default/deployments
```

- **Pods**:
```bash
curl -s http://127.0.0.1:8001/api/v1/namespaces/default/pods
```

- **Services**:
```bash
curl -s http://127.0.0.1:8001/api/v1/namespaces/default/services
```

- **Nodes**:
```bash
curl -s http://127.0.0.1:8001/api/v1/nodes
```

## 2. Gestione VM Proxmox

### 2.1 Autenticazione

L'autenticazione alle API Proxmox richiede un token API strutturato come segue:

(! nei dati )

```bash
PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>
```

### 2.2 Operazioni sulle VM

#### Creazione VM
```bash
curl -X POST "https://<PROXMOX_BASE_URL>/api2/json/nodes/<NODE_NAME>/qemu" \
     -H "Authorization: PVEAPIToken>" \
     -d "vmid=100" \
     -d "cores=2" \
     -d "memory=4096"
```

#### Gestione VM Esistente

- **Eliminazione**:
```bash
curl -X DELETE "https://<PROXMOX_BASE_URL>/api2/json/nodes/<NODE_NAME>/qemu/<VMID>" \
     -H "Authorization: PVEAPIToken"
```

- **Modifica Configurazione**:
```bash
curl -X POST "https://<PROXMOX_BASE_URL>/api2/json/nodes/<NODE_NAME>/qemu/<VMID>/config" \
     -H "Authorization: PVEAPIToken" \
     -d "cores=4" \
     -d "memory=8192"
```

- **Controllo Stato** (start/stop/reset):
```bash
curl -X POST "https://<PROXMOX_BASE_URL>/api2/extjs/nodes/<NODE_NAME>/qemu/<VMID>/status/start" \
     -H "Authorization: PVEAPIToken"
```

- **Clonazione**:
```bash
curl -X POST "https://<PROXMOX_BASE_URL>/api2/json/nodes/<NODE_NAME>/qemu/<SOURCE_VMID>/clone" \
     -H "Authorization: PVEAPIToken" \
     -d "newid=200" \
     -d "full=1" \
     -d "storage=<STORAGE_NAME>"
```

### 2.3 Monitoraggio VM

#### Status VM
```bash
curl -X GET "https://<PROXMOX_BASE_URL>/api2/extjs/nodes/<NODE_NAME>/qemu/<VMID>/status/current" \
     -H "Authorization: PVEAPIToken"
```

#### Metriche Cluster
```bash
curl -X GET "https://<PROXMOX_BASE_URL>/api2/extjs/cluster/resources?type=vm" \
     -H "Authorization: PVEAPIToken"