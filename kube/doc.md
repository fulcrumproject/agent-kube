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
Per creare un TCP, è necessario prima generare un token di accesso:

```bash
kubectl create token api-access -n kube-system
```

Esistono due modalità per creare un TCP:

1. **Chiamata diretta con token**:
```bash
curl -X POST https://<KAMAJI_API_SERVER>/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>" \
  -H "Content-Type: application/yaml" \
  --data-binary @tenant-control-plane.yaml 
```

2. **Utilizzo del proxy kubectl** 
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
curl -X DELETE https://<KAMAJI_API_SERVER>/apis/kamaji.clastix.io/v1alpha1/namespaces/<NAMESPACE>/tenantcontrolplanes/<TENANT_NAME> \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

#### Aggiornamento TCP

1. **Modifica parametro singolo** (esempio: modifica repliche):
```bash
curl -X PATCH https://<KAMAJI_API_SERVER>/apis/kamaji.clastix.io/v1alpha1/namespaces/<NAMESPACE>/tenantcontrolplanes/<TENANT_NAME> \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"spec": {"controlPlane": {"deployment": {"replicas": 3}}}}'
```

2. **Sostituzione configurazione completa**:
```bash
curl -X PUT https://<KAMAJI_API_SERVER>/apis/kamaji.clastix.io/v1alpha1/namespaces/<NAMESPACE>/tenantcontrolplanes/tenant-test \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  --data-binary @tenantcontrolplane.json
```

### 1.3 Monitoraggio Risorse

#### Query Informazioni Cluster

- **Lista TCP**:
```bash
curl -s https://<KAMAJI_API_SERVER>/apis/kamaji.clastix.io/v1alpha1/namespaces/<NAMESPACE>/tenantcontrolplanes \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

- **Dettagli TCP Specifico**:
```bash
curl -s https://<KAMAJI_API_SERVER>/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes/<TENANT_NAME> \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

- **Deployments**:
```bash
curl -s https://<KAMAJI_API_SERVER>/apis/apps/v1/namespaces/default/deployments \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

- **Pods**:
```bash
curl -s https://<KAMAJI_API_SERVER>/api/v1/namespaces/default/pods \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

- **Services**:
```bash
curl -s https://<KAMAJI_API_SERVER>/api/v1/namespaces/default/services \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

- **Nodes**:
```bash
curl -s https://<KAMAJI_API_SERVER>/api/v1/nodes \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

## 2. Gestione VM Proxmox

### 2.1 Autenticazione

L'autenticazione alle API Proxmox richiede un token API strutturato come segue:

> **⚠️ Importante**: Il token ID deve essere separato dal realm utilizzando il carattere `!`

```bash
PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>
```

### 2.2 Operazioni sulle VM

#### Lista Nodi
```bash
curl -X GET -k 'https://<PROXMOX_HOST>:8006/api2/json/nodes' \
     -H 'Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>'
```

#### Informazioni VM
```bash
curl -X GET -k 'https://<PROXMOX_HOST>:8006/api2/json/nodes/<NODE_NAME>/qemu/<VMID>/<SPEC>' \
     -H 'Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>'
```

> **Note**: Valori possibili per `<SPEC>`: config, cloudinit, pending, status, unlink, vncproxy, termproxy, migrate, resize, move, rrd, rrddata, monitor, agent, snapshot, spiceproxy, sendkey, firewall, mtunnel, remote_migrate

#### Creazione VM

> **⚠️ Attenzione**: 
> - Se l'ID VM esiste già, l'API risponderà con errore 500
> - In caso di successo, viene restituito un UPID (Unique Process ID)

```bash
curl -X POST -k "https://<PROXMOX_HOST>:8006/api2/json/nodes/<NODE_NAME>/qemu" \
     -H 'Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>' \
     -d "vmid=<VM_ID>" \
     -d "cores=2" \
     -d "memory=4096"
```

#### Gestione VM Esistente

- **Eliminazione**:
```bash
curl -X DELETE -k "https://<PROXMOX_HOST>:8006/api2/json/nodes/<NODE_NAME>/qemu/<VMID>" \
     -H "Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>"
```

- **Modifica Configurazione**:
```bash
curl -X POST "https://<PROXMOX_HOST>:8006/api2/json/nodes/<NODE_NAME>/qemu/<VMID>/config" \
     -H "Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>" \
     -d "cores=4" \
     -d "memory=8192"
```

- **Controllo Stato** (start/stop/reset):
```bash
curl -X POST "https://<PROXMOX_HOST>:8006/api2/extjs/nodes/<NODE_NAME>/qemu/<VMID>/status/start" \
     -H "Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>"
```

- **Clonazione**:
```bash
curl -X POST "https://<PROXMOX_HOST>:8006/api2/json/nodes/<NODE_NAME>/qemu/<SOURCE_VMID>/clone" \
     -H "Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>" \
     -d "newid=<NEW_VMID>" \
     -d "full=1" \
     -d "storage=<STORAGE_NAME>"
```

### 2.3 Monitoraggio VM

#### Status VM
```bash
curl -X GET "https://<PROXMOX_HOST>:8006/api2/extjs/nodes/<NODE_NAME>/qemu/<VMID>/status/current" \
     -H "Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>"
```

#### Metriche Cluster
```bash
curl -X GET "https://<PROXMOX_HOST>:8006/api2/extjs/cluster/resources?type=vm" \
     -H "Authorization: PVEAPIToken=<USER>@<REALM>!<TOKEN_ID>=<SECRET>"