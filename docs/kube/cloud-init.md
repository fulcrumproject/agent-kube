```bash 
qm set 20002 --cicustom "user=$(echo '#cloud-config
runcmd:
  - echo "FUNZIONAAAA"
' | base64 -w0)"
```