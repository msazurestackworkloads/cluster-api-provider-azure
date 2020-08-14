# Azure Stack

To deploy a cluster using [Azure Stack Hub](https://github.com/msazurestackworkloads/cluster-api-provider-azure), create a cluster configuration with the [azure stack template](../../templates/cluster-template-azure-stack.yaml).

## Upload VHD to Azure Stack Hub
If this is the first time setting up, create your own VHD using [image-builder](https://github.com/msazurestackworkloads/image-builder/tree/azure-stack-vhd-18.04). Then in the Azure Stack Hub Admin Portal, upload a platform VM image with the following parameters: 
- Publisher: AzureStack
- Offer: Test
- OS type: Linux
- SKU: capz-test-1804
- Version: 1.0.0
- OS disk blob URI: Insert image builder output VHD URI here

## Set environment variables

### Azure cloud settings
```bash
export AZURE_ARM_ENDPOINT="https://management.redmond.ext-n31r1203.masd.stbtest.microsoft.com"
export AZURE_LOCATION="redmond"
export AZURE_ENVIRONMENT=AzureStackCloud
export AZURE_ENVIRONMENT_FILEPATH=”/etc/kubernetes/azurestackcloud.json”
export IDENTITY_SYSTEM=adfs
if [ "$IDENTITY_SYSTEM" = "adfs" ]; then
export IDENTITY_TENANT_ID="adfs"
else
export IDENTITY_TENANT_ID=${AZURE_TENANT_ID}
fi
```
Azure Stack offers both Azure Active Directory and ADFS identity providers. Environment variable `IDENTITY_SYSTEM` can be either `azure_ad` or `adfs`.

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
export KUBERNETES_VERSION="v1.18.2"
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
## Build docker image 
```bash
export REGISTRY="gcr.io"
export PULL_POLICY=IfNotPresent 
make docker-build
```

## Create management cluster 
```bash
export EXP_MACHINE_POOL=true
make create-management-cluster
```
## Set workload cluster template manifest

### AzureStackCloud json
Package go-autorest defines a variable of type [Environment](https://godoc.org/github.com/Azure/go-autorest/autorest/azure#Environment) for each Azure cloud (Public, China, Germany, US Gov).
 
In Azure Stack's case, the environment has to be dynamically determined by querying [Azure Stack's metadata endpoint](https://docs.microsoft.com/en-us/azure-stack/user/azure-stack-version-profiles-go?view=azs-2005#how-to-use-go-sdk-profiles-on-azure-stack-hub). Composing [this list](https://github.com/kubernetes/cloud-provider-azure/blob/master/docs/cloud-provider-config.md#azure-stack-configuration) is required in order to indicate to azure cloud provider what endpoints to target.

Use the following bash script to generate the azurestackcloud json. Paste the output of the bash script into the workload cluster template, replacing the corresponding azurestackcloud file content placeholders.

Usage: $> azs_endpoints.sh local azure.external 
```bash
#!/bin/bash

LOCATION=$1
FQDN=$2
 
METADATA=$(mktemp)
 
MANAGEMENT="https://management.${LOCATION}.${FQDN}/"
curl -o ${METADATA} -k "${MANAGEMENT}metadata/endpoints?api-version=1.0"

NAME="AzureStackCloud"
PORTALURL="https://portal.${LOCATION}.${FQDN}/"
SRVMANAGEMENT="$(jq -r '.authentication.audiences | .[0]' "$METADATA")"
AD="$(jq -r .authentication.loginEndpoint "$METADATA" | sed -e 's/adfs\/*$//')"
GALLERY="$(jq -r .galleryEndpoint "$METADATA")"
GRAPH="$(jq -r .graphEndpoint "$METADATA")"
STORAGE="${LOCATION}.${FQDN}",
KEYVAULT="vault.${LOCATION}.${FQDN}"
RESOURCEMANAGER="cloudapp.${FQDN}"
 
jq -n \
--arg NAME "$NAME" \
--arg PORTALURL "$PORTALURL" \
--arg SRVMANAGEMENT "$SRVMANAGEMENT" \
--arg MANAGEMENT "$MANAGEMENT" \
--arg AD "$AD" \
--arg GALLERY "$GALLERY" \
--arg GRAPH "$GRAPH" \
--arg STORAGE "$STORAGE" \
--arg KEYVAULT "$KEYVAULT" \
--arg RESOURCEMANAGER "$RESOURCEMANAGER" \
'{
name: $NAME,
managementPortalURL: $PORTALURL,
serviceManagementEndpoint: $SRVMANAGEMENT,
resourceManagerEndpoint: $MANAGEMENT,
activeDirectoryEndpoint: $AD,
galleryEndpoint: $GALLERY,
graphEndpoint: $GRAPH,
storageEndpointSuffix: $STORAGE,
keyVaultDNSSuffix: $KEYVAULT,
resourceManagerVMDNSSuffix: $RESOURCEMANAGER
}'
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