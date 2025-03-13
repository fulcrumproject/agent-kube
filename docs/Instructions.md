# Agent Kubernetes

## 1. Installazione e Affiliazione Nuovo Nodo Control Plane

### 1.1 Creazione VM su Proxmox

#### 1.1.1 Configurazione tramite API Proxmox
La VM verrà creata nel cluster principale utilizzando il template. Parametri richiesti:

```yaml
JOIN_URL: <control-plane-endpoint>:<port>
JOIN_TOKEN: <token>
JOIN_TOKEN_CERT_KEY: <key>
JOIN_TOKEN_CACERT_HASH: sha256:<hash>
JOIN_ASCP: 1
KUBERNETES_VERSION: v1.30.5
```

#### 1.1.2 Verifica dello Stato
- Verificare tramite API Kubernetes che la macchina sia correttamente online e funzionante
- Controllare lo stato del nodo nel cluster

#### 1.1.3 Output Atteso
- Restituire i parametri di configurazione del nuovo control plane
- Confermare il successo dell'installazione

## 2. Aggiunta Worker Node

### 2.1 Parametri di Configurazione

```yaml
JOIN_URL: <control-plane-endpoint>:<port>
JOIN_TOKEN: <token>
JOIN_TOKEN_CACERT_HASH: sha256:<hash>
JOIN_ASCP: 1
KUBERNETES_VERSION: v1.30.5
```


