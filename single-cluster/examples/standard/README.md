# Deploying the Data Forwarder in a Single Cluster

This example shows you how to setup the data forwarder in your cluster, connect to an in-cluster Prometheus server and send container data to Densify on an hourly basis.

Edit the `configmap.yaml` file, then create the config map from it. Test the data forwarder setup by creating a pod to ensure that data is sent to Densify before enabling the CronJob to run data collection every hour.

## Prerequisites

You need to setup the following prerequistes before deploying the data forwarder:

1. A Linux environment with `bash` and [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

2. Download the following files and save them to your workspace:
	- configmap.yaml
	- cronjob.yaml
	- pod.yaml

## Deploying the Data Forwarder

1. Edit `configmap.yaml` to add the details of your Densify instance and the Prometheus server.

2. Create the config map in Kubernetes:

    `kubectl create -f configmap.yaml -n <namespace>`

3. Test your configuration using the test pod:

    `kubectl create -f pod.yaml -n <namespace>`

	Once the collected container data is sent to Densify, the pod exits.

4. Review the log for the container

	`kubectl logs densify -n <namespace>`

	You should see lines similar to the following, near the end of the log:

	> {"level":"info","pkg":"default","time":1651699421230540,"caller":"src/densify.com/forwarderv2/files.go:88","goid":1,"message":"zipping gke_cluster.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 22 files; hpa - 4 files; rq - 7 files; crq - 0 files; total - 87 files"}

	> {"level":"info","pkg":"default","file":"data/gke_cluster.zip","time":1651699421321196,"caller":"src/densify.com/forwarderv2/main.go:47","goid":1,"message":"file uploaded successfully"}

	The exact number of files in each subfolder and the total number of files depends on:
	- The data forwarder's `collection.include` setting;
	- Configuration of `kube-state-metrics`;
	- Details of the Kubernetes cluster, from which the data is being collected (i.e what is defined/running in the cluster).

	If you use the default `collection.include` configuration, at the very least you should see files for the cluster, container and node. Other files are cluster-specific.
	If the number of files is lower than expected, there may be issues sending container data to Densify and you need to review the log for more details and contact support@densify.com for help.

5. Cleanup

    `kubectl delete -f pod.yaml -n <namespace>`

6. Create the CronJob using `cronjob.yaml`

    `kubectl create -f cronjob.yaml -n <namespace>`

	The CronJob will run and send data to Densify hourly. You need to adjust the CronJob schedule to run on the same `collection.interval_size`, defined in the config map.
