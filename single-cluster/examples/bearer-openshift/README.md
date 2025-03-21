# Configuring the Data Forwarder to use Authenticated Prometheus using Bearer Tokens (OpenShift)

This example shows you how to setup the data forwarder to connect to an authenticated Prometheus server to collect data from a single cluster. This is the default configuration for the OpenShift monitoring setup, where Prometheus server authentication uses bearer tokens.

If you have tried the CronJob example and received an `x509` or `403` error, then you likely need to use this setup.

To configure the data forwarder with authenticated Prometheus, edit the `configmap.yaml` file, then create the config map from it. You also need to create a service account, cluster role, and cluster role binding. To test the data forwarder setup, create a pod to ensure that data is sent to Densify before enabling the CronJob to run data collection every hour.

## Prerequisites

You need to setup the following prerequistes before deploying the data forwarder:

1. A Linux environment with `bash` and one of the following utilities:

   - For OpenShift clusters, use [oc, OpenShift CLI](https://docs.openshift.com/container-platform/4.15/cli_reference/openshift_cli/getting-started-cli.html).

   or:

   - For all other clusters, use [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)

2. Download the following files and save them to your workspace:
	- clusterrole.yaml
	- clusterrolebinding.yaml
	- configmap.yaml
	- cronjob.yaml
	- pod.yaml
	- serviceaccount.yaml

## Deploying the Data Forwarder

1. Edit `configmap.yaml` to add the details of your Densify instance and the Prometheus server.

2. Create the service account:

    `oc create -f serviceaccount.yaml -n <namespace>`

	or

    `kubectl create -f serviceaccount.yaml -n <namespace>`

3. Create the cluster role:

    `oc create -f clusterrole.yaml`

	or

    `kubectl create -f clusterrole.yaml`

4. Edit `clusterrolebinding.yaml` to set the namespace, in which to run the data forwarder (in two locations!)

5. Create the cluster role bindings:

    `oc create -f clusterrolebinding.yaml`

	or

    `kubectl create -f clusterrolebinding.yaml`

6. Create the config map:

    `oc create -f configmap.yaml -n <namespace>`

	or

    `kubectl create -f configmap.yaml -n <namespace>`
	
7. Test your configuration using the test pod:

    `oc create -f pod.yaml -n <namespace>`

	or

    `kubectl create -f pod.yaml -n <namespace>`

	Once the collected container data is sent to Densify, the pod exits.

8. Review the log for the container:

    `oc logs densify -n <namespace>`

	or

	`kubectl logs densify -n <namespace>`

	You should see lines similar to the following, near the end of the log:

	> {"level":"info","pkg":"default","time":1651699421230540,"caller":"src/densify.com/forwarderv2/files.go:88","goid":1,"message":"zipping os_cluster.zip, contents: cluster - 21 files; container - 11 files; node - 17 files; rq - 7 files; crq - 7 files; `total - 63 files`"}
	
	> {"level":"info","pkg":"default","file":"data/os_cluster.zip","time":1651699421321196,"caller":"src/densify.com/forwarderv2/main.go:47","goid":1,"message":"`file uploaded successfully`"}

	The exact number of files in each subfolder and the total number of files depends on:
	- The data forwarder's `collection.include` setting;
	- Configuration of `kube-state-metrics`;
	- Details of the Kubernetes cluster, from which the data is being collected (i.e what is defined/running in the cluster).

	If you use the default `collection.include` configuration, at the very least you should see files for the cluster, container, node and HPA. Other files are cluster-specific.
	If the number of files is lower than expected, there may be issues sending container data to Densify and you need to review the log for more details and contact support@densify.com for help. 

9. Cleanup

    `oc delete -f pod.yaml -n <namespace>`

	or
	
    `kubectl delete -f pod.yaml -n <namespace>`


10. Create the CronJob using `cronjob.yaml`

    `oc create -f cronjob.yaml -n <namespace>`

	or

    `kubectl create -f cronjob.yaml -n <namespace>`

	The CronJob will run and send data to Densify hourly. You need to adjust the CronJob schedule to run on the same `collection.interval_size`, defined in the config map.
