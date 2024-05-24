# Configuring the Data Forwarder to use Prometheus or an Observability Platform using Basic Authentication

This example shows you how to setup the data forwarder to connect to Prometheus or an observability platform that supports the Prometheus API with HTTP basic authentication and sends container data from multiple clusters to Densify on an hourly basis.

## Prerequisites

Setup the following prerequistes before deploying the data forwarder:

1. A Linux environment with `bash` and [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

2. Configure the collection of Prometheus metrics from each cluster. Ensure that all of the metrics required by Densify are collected. See [Required Prometheus Metrics](../../../docs).

3. Configure each cluster name, from which you are collecting data, on your Prometheus server or collector. Both the label name and the value are required for `configmap.yaml`.

4. Download the following files and save them to your workspace:
   - configmap.yaml
   - pod.yaml
   - cronjob.yaml

## Deploying the Data Forwarder

1. Modify `configmap.yaml` to point to your Densify instance and to your observability platform.

2. Create the config map in Kubernetes:

    `kubectl create -f configmap.yaml -n <namespace>`

3. Test your configuration using the test pod:

    `kubectl create -f pod.yaml -n <namespace>`

	Once the collected container data is sent to Densify, the pod exits.

4. Review the log for the container:

	`kubectl logs densify -n <namespace>`

	You should see lines similar to the following, near the end of the log:

	> {"level":"info","pkg":"default","time":1704496040349770,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-1, zipping cluster-1.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; `total - 54 files`"}

	> {"level":"info","pkg":"default","time":1704496040449763,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-2, zipping cluster-2.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; `total - 54 files`"}

	> {"level":"info","pkg":"default","file":"data/cluster-1.zip","time":1704496040616014,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"`file uploaded successfully`"}

	> {"level":"info","pkg":"default","file":"data/cluster-2.zip","time":1704496040666046,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"`file uploaded successfully`"}

	The exact number of files in each subfolder and the total number of files will depend on:
	- The data forwarder's `collection.include` setting;
	- Configuration of `kube-state-metrics`;
	- Details of the Kubernetes cluster, from which the data is being collected (i.e what is defined/running in the cluster).

	If you use the default `collection.include` configuration, at the very least you should see files for the cluster, container and node. Other files are cluster-specific.

	If the number of files is lower than expected, there may be issues sending container data to Densify and you need to review the log for more details and contact support@densify.com for help.

5. Cleanup

    `kubectl delete -f pod.yaml -n <namespace>`


6. Create the CronJob using `cronjob.yaml`

    `kubectl create -f cronjob.yaml -n <namespace>`

	The CronJob runs and sends the collected container data to Densify hourly. You need to adjust the CronJob schedule to run on the same `collection.interval_size`, defined in the config map.
