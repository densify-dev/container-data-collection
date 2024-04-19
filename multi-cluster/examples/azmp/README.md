# Azure Monitor Managed Prometheus

## Pre-requisites

Steps 1 and 4 require a Linux environment with `bash` and the two utilities:

* [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli)
* [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
* [jq](https://jqlang.github.io/jq/)
* base64 - pre-installed in most Linux distros as part of the `coreutils` package

## Scope

[Azure Monitor managed service for Prometheus](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-overview) is an observability platform, usually used when rolling out Azure AKS clusters. It can be used in one of two ways.

### Remote-write from self-managed Prometheus

If you opt to roll out your own Prometheus stack and use its [remote-write protocol to an Azure Monitor Workspace](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/remote-write-prometheus), then follow the general instructions [here](../../../docs/metrics.md) as for which exporters to deploy (and which metrics to collect).

### Prometheus Enabled AKS Cluster

Azure allows you to enable metrics collection from your AKS cluster and send these to your Azure Monitor Prometheus workspace. This is as simple as one check box and the workspace name in the `Managed Prometheus` section of the `Monitoring` tab:

![Create AKS cluster UI](./create-cluster.png)

However, the default Azure AKS monitoring stack does not collect all of the metrics required for Densify container data collection. To make sure all of the required metrics are collected, perform step 1:

1. Edit the first four lines lines of [setup-azmp-aks-cluster.sh](./setup-azmp-aks-cluster.sh) like this:

> CLUSTER_NAME=<my_azure_aks_cluster_name>

> RESOURCE_GROUP=<my_azure_aks_cluster_resourcegroup>

> AZMP_ALREADY_ENABLED=1 # leave as 1 if you have enabled Managed Prometheus in Azure Portal UI, change to 0 if not

> ADD_TO_KUBECONFIG=1 # leave as 1 if it's a new cluster which was not added yet to your kubeconfig, change to 0 if that's already done

Run the shell script using
`./setup-azmp-aks-cluster.sh`
and verify it completes successfully. The two steps of disabling Azure Monitor Metrics (if already enabled) and re-enabling these may take each up to a few minutes.

## Get Azure Monitor Prometheus Workspace Details

Now, go in the Azure Portal to `Monitor -> Managed Prometheus`, and select the relevant workspace.

2. You'll see a `Query endpoint` value (ending with `prometheus.monitor.azure.com`). Copy it and paste it into `configmap.yaml` under `prometheus.url.host`. Save the file.

3. You'll also see `JSON view`, click it and you'll see a `Resource ID` value. Copy it and paste it into [register-app-create-secret.sh](./register-app-create-secret.sh) as the value of `AZMON_WORKSPACE_RESOURCE_URI`. Save the file.

## Register an Entra service principal and create secret

In order to be able to run PromQL queries with Azure Managed Prometheus API, we need to register a Microsoft Entra (formerly Azure AD) service principal and give it the relevant role. This is described [here](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql) and [here](https://learn.microsoft.com/en-us/azure/azure-monitor/logs/api/register-app-for-token?tabs=cli).

4. You have already edited [register-app-create-secret.sh](./register-app-create-secret.sh) in step 3. This script:

* creates the service principal
* assigns to it the relevant role
* creates a yaml file for a Kubernetes secret with the service principal ID

Run the shell script `register-app-create-secret.sh` and verify it completes successfully.

5. Now, create the Azure Monitor secret in Kubernetes.
    
    `kubectl create -f azmon-secret.yaml -n <namespace>`

## Proceed to deploy the forwarder

6. You have already edited `configmap.yaml` with your AzMP workspace. Add your Densify instance and cluster identifiers to it.

7. Create the config map in Kubernetes
    
    `kubectl create -f configmap.yaml -n <namespace>`
	
8. Create the pod using `pod.yaml`
    
    `kubectl create -f pod.yaml -n <namespace>`
	
9. Review the log for the container
	
	`kubectl logs densify -n <namespace>`
	
	You should see lines similar to the following, near the end of the log:

	> {"level":"info","pkg":"default","time":1704496040349770,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-1, zipping cluster-1.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; total - 54 files"}

	> {"level":"info","pkg":"default","time":1704496040449763,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-2, zipping cluster-2.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; total - 54 files"}

	> {"level":"info","pkg":"default","file":"data/cluster-1.zip","time":1704496040616014,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"file uploaded successfully"}

	> {"level":"info","pkg":"default","file":"data/cluster-2.zip","time":1704496040666046,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"file uploaded successfully"}

	The exact number of files - under each subfolder and total - depends on the Data Forwarder `include_list` configuration, kube-state-metrics configuration and what is defined/running in the Kubernetes cluster we collect data for. If we use the default `include_list` configuration (empty value means collect all), we should see non-zero number of files at least for cluster, container, node and hpa. The other are cluster-specific.
	If the numbers are lower than expected, you probably have issues with sending container data to Densify and need to review the rest of the log and contact Densify support. Otherwise, you can move on to the next step.
	
	Once the collected container data is sent to Densify, the pod exits.

10. Cleanup

    `kubectl delete -f pod.yaml -n <namespace>`

11. Create the cronjob using `cronjob.yaml`
    
    `kubectl create -f cronjob.yaml -n <namespace>`
	
	The cronjob runs and sends the collected container data to Densify hourly. You need to schedule the pod to run at the same `collection.interval_size` that is configured for data collection, as defined in the config map.
