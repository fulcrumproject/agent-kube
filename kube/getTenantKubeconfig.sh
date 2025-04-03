
## GET KAMAJI JOIN COMMAND

#get tenant kubeconfig 
kubectl get secret tenant-test-admin-kubeconfig -n default -o jsonpath='{.data.admin\.conf}' | base64 -d > tenant-test.kubeconfig

#kubeadm init su cluster kamaji
kubeadm init 

#kubeadm join su cluster proxmox
#comando completo fornito in risposta al comando kubeadmin init
kubeadm join 


#questo in teoria dovrebbe essere sostituito da yaki 
curl -sfL https://goyaki.clastix.io | sudo bash -s init
# dovrebbe restituire 
    # JOIN_URL=<join-url>
    # JOIN_TOKEN=<token>
    # JOIN_TOKEN_CERT_KEY=<certificate-key>
    # JOIN_TOKEN_CACERT_HASH=sha256:<discovery-token-ca-cert-hash>
# per poter fare la join al cluster  

kubeadm join 172.30.232.66:6443 --token trwpz3.ufe0npfxr5m8ob9s --discovery-token-ca-cert-hash sha256:d900d6f505f17569537b03fe8bec8b67f1d051886c918c50c19e25d988ecdce6