# Basic Auth Prometheus / Observability Platform

This example shows you how to setup the Data Forwarder to connect to Prometheus or an observability platform supporting Prometheus API with HTTP basic authentication (e.g. Grafana Cloud), and send container data to Densify on an hourly basis. You need to edit the `configmap.yaml` file, then create the config map to pass the settings to `config.yaml`. To test the Data Forwarder setup, create a pod to ensure that data is sent to Densify before enabling the cronjob to run data collection every hour.

## Pre-requisites

All steps require [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)

## Steps

1. Modify `configmap.yaml` to point to your Densify instance and to the observability platform.

2. Create the config map in Kubernetes
    
    `kubectl create -f configmap.yaml -n <namespace>`
	
3. Create the pod using `pod.yaml`
    
    `kubectl create -f pod.yaml -n <namespace>`
	
4. Review the log for the container
	
	`kubectl logs densify -n <namespace>`
	
	You should see lines similar to the following, near the end of the log:

	> {"level":"info","pkg":"default","time":1704496040349770,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-1, zipping cluster-1.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; total - 54 files"}

	> {"level":"info","pkg":"default","time":1704496040449763,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-2, zipping cluster-2.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; total - 54 files"}

	> {"level":"info","pkg":"default","file":"data/cluster-1.zip","time":1704496040616014,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"file uploaded successfully"}

	> {"level":"info","pkg":"default","file":"data/cluster-2.zip","time":1704496040666046,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"file uploaded successfully"}

	The exact number of files - under each subfolder and total - depends on the Data Forwarder `include_list` configuration, kube-state-metrics configuration and what is defined/running in the Kubernetes cluster we collect data for. If we use the default `include_list` configuration (empty value means collect all), we should see non-zero number of files at least for cluster, container, node and hpa. The other are cluster-specific.
	If the numbers are lower than expected, you probably have issues with sending container data to Densify and need to review the rest of the log and contact Densify support. Otherwise, you can move on to the next step.
	
	Once the collected container data is sent to Densify, the pod exits.
		
5. Create the cronjob using `cronjob.yaml`
    
    `kubectl create -f cronjob.yaml -n <namespace>`

The cronjob runs and sends the collected container data to Densify hourly.
You need to schedule the pod to run at the same `collection.interval_size` that is configured for data collection, as defined in the config map.
