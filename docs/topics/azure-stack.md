# Azure Stack

To deploy a cluster using [Azure Stack Hub](https://github.com/msazurestackworkloads/cluster-api-provider-azure), create a cluster configuration with the [azure stack template](../../templates/cluster-template-azure-stack.yaml).


## Set environment variables

### Azure cloud settings
```bash
export AZURE_ARM_ENDPOINT="https://management.redmond.ext-n31r1203.masd.stbtest.microsoft.com"
export AZURE_LOCATION="redmond"
export AZURE_ENVIRONMENT=AzureStackCloud
export AZURE_ENVIRONMENT_FILEPATH=”/etc/kubernetes/azurestackcloud.json”
export IDENTITY_SYSTEM=adfs
```
Azure Stack offers both Azure Active Directory and ADFS authentication. Environment variable `IDENTITY_SYSTEM` can be either `azure_ad` or `adfs`.

### Azure Service Principal 
```bash
export AZURE_TENANT_ID="<Tenant>"
export AZURE_CLIENT_ID="<AppId>"
export AZURE_CLIENT_SECRET="<Password>"
export AZURE_SUBSCRIPTION_ID="<SubscriptionId>"

export AZURE_SUBSCRIPTION_ID_B64="$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
export AZURE_TENANT_ID_B64="$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_ID_B64="$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_SECRET_B64="$(echo -n "$AZURE_CLIENT_SECRET" | base64 | tr -d '\n')"
```

### Cluster settings
```bash
export CLUSTER_NAME="capz-cluster"
export AZURE_RESOURCE_GROUP=${CLUSTER_NAME}
export AZURE_VNET_NAME=${CLUSTER_NAME}-vnet
```
### Machine settings
```bash
export CONTROL_PLANE_MACHINE_COUNT=1
export AZURE_CONTROL_PLANE_MACHINE_TYPE="Standard_DS2_v2"
export AZURE_NODE_MACHINE_TYPE="Standard_DS2_v2"
export WORKER_MACHINE_COUNT=2
export KUBERNETES_VERSION="v1.17.8"
```

### Generate SSH key
If you want to provide your own key, skip this step and set AZURE_SSH_PUBLIC_KEY to your existing file.
```bash
SSH_KEY_FILE=.sshkey
rm -f "${SSH_KEY_FILE}" 2>/dev/null
ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
echo "Machine SSH key generated in ${SSH_KEY_FILE}"
export AZURE_SSH_PUBLIC_KEY=$(cat "${SSH_KEY_FILE}.pub" | base64 | tr -d '\r\n')
```

## Build docker image after changes to codebase
```bash
export REGISTRY="gcr.io"
export PULL_POLICY=IfNotPresent 
make docker-build
```

## Create management cluster 
```bash
make create-management-cluster
```

## Create workload cluster 
```bash
export CLUSTER_TEMPLATE=cluster-template-azure-stack.yaml
make create-workload-cluster
```

## Debug
### Retrieve CAPZ logs
```bash
kubectl logs deployment/capz-controller-manager -n capz-system --all-containers=true > stack.log
```

### ssh into virtual machine
```bash
ssh -i repo/cluster-api-provider-azure/.sshkey capi@<ipaddress>
cat /var/log/cloud-init-output.log > init.log
sudo journalctl -u kubelet -l --no-pager > kubelet.log
```