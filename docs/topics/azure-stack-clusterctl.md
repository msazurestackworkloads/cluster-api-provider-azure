# Azure Stack clusterctl

## Development Workflow

### Setup
To deploy a cluster to Azure Stack Hub using [clusterctl](https://cluster-api.sigs.k8s.io/clusterctl/overview.html) for the first time, follow [Getting Started](https://cluster-api.sigs.k8s.io/clusterctl/developers.html#getting-started) to build clusterctl binary, create a `clusterctl-settings.json` file, and run the local-overrides hack. Then copy the Azure Stack flavor template to the local override repository using: 

```bash
cp repo/cluster-api-provider-azure/templates/cluster-template-azure-stack.yaml ~/.cluster-api/overrides/infrastructure-azure/v0.4.0/
```

### Create kind cluster

Cluster API requires an existing Kubernetes cluster accessible via kubectl; during the installation process the Kubernetes cluster will be transformed into a management cluster by installing the Cluster API provider components. 
```bash
kind create cluster
kind load docker-image gcr.io/cluster-api-azure-controller-amd64:dev
```

### Initalize management cluster
Transform the Kubernetes cluster into a management cluster by using `clusterctl init`. The command accepts as input a list of providers to install. 
```bash
clusterctl init --core cluster-api:v0.3.0 --bootstrap kubeadm:v0.3.0 --control-plane kubeadm:v0.3.0 --infrastructure azure:v0.4.0
```

### Create workload cluster
Once the management cluster is ready, you can create your workload cluster. The `clusterctl config cluster` command returns a YAML template for creating a workload cluster. 

Set the required [Azure Stack environment variables](./azure-stack.md). 

Generate the cluster configuration, either by flavor or directly from file.

To generate by flavor: 
```bash
clusterctl config cluster capz-cluster2 --kubernetes-version v1.17.8 --control-plane-machine-count 1 --worker-machine-count 1 --flavor azure-stack > my-cluster.yaml
```
To generate directly from file: 
```bash
clusterctl config cluster capz-cluster2 --kubernetes-version v1.17.8 --control-plane-machine-count 1 --worker-machine-count 1 --from repo/cluster-api-provider-azure/templates/cluster-template-azure-stack.yaml > my-cluster.yaml
```
This creates a YAML file named `my-cluster.yaml` with a predefined list of Cluster API objects; Cluster, Machines, Machine Deployments, etc. 

When ready, run the following command to apply the cluster manifest, creating a workload cluster on Azure Stack Hub:
```bash
kubectl apply -f my-cluster.yaml
```

### Accessing the workload cluster
The cluster will now start provisioning. You can check status with: 
```bash
kubectl get cluster --all-namespaces
```
After the first control plane node is up and running, we can retrieve the workload cluster Kubeconfig: 
```bash
kubectl get secrets capz-cluster2-kubeconfig -o json | jq -r .data.value | base64 --decode > ./kubeconfig
```

### Deploy a CNI solution
Calico is used here as an example. 
```bash
kubectl --kubeconfig=./kubeconfig apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/addons/calico.yaml
```

After a short while, our nodes should be running and in `Ready` state, let's check the status using `kubectl get nodes`
```bash
kubectl --kubeconfig=./kubeconfig get nodes
```