This example shows you how to setup the Data Forwarder to connect to an authenticated Prometheus configuration. This is typically the case for OpenShift default monitoring setup, where the Prometheus server is setup for authentication, even if the internal kubernetes service name is used. If you have tried the CronJob example and received an x509 or 403 error, then you likely need to use this setup. 

To configure the Data Forwarder with an authenticated Prometheus, you need to edit the `configmap.yaml` file, then create the config map to pass the settings to `config.properties`. In addition, create a service account, cluster role, and cluster role binding. To test the Forwarder setup, create a pod to ensure that data is sent to Densify before enabling the cron job to run data collection every hour.

1. Modify `configmap.yaml` to point to your Densify instance and the Prometheus server.
2. Create the service account:

    `kubectl create -f serviceaccount.yaml`

3. Create the cluster role:

    `kubectl create -f clusterrole.yaml`

4. Modify the cluster role binding to set the namespace being used to run the forwarder:

	`namespace: <namespace using for Forwarder>`

5. Create the cluster role binding:

    `kubectl create -f clusterrolebinding.yaml`

6. Create the config map:

    `kubectl create -f configmap.yaml -n <namespace>`
	
7. Create the pod to test the Forwarder using `pod.yaml`:

    `kubectl create -f pod.yaml -n <namespace>`
	
8. Review the log for the container

	`kubectl logs densify -n <namespace>`
	
	You should see lines similar to the following, near the end of the log:
	
	> {"level":"info","pkg":"default","time":1651699421230540,"caller":"src/densify.com/forwarderv2/files.go:88","goid":1,"message":"zipping os_cluster.zip, contents: cluster - 21 files; container - 11 files; node - 17 files; rq - 7 files; crq - 7 files; total - 63 files"}
	
	> {"level":"info","pkg":"default","file":"data/os_cluster.zip","time":1651699421321196,"caller":"src/densify.com/forwarderv2/main.go:47","goid":1,"message":"file uploaded successfully"}
	
	The exact number of files - under each subfolder and total - depends on the Data Forwarder `include_list` configuration, kube-state-metrics configuration and what is defined/running in the Kubernetes cluster we collect data for. If we use the default `include_list` configuration (empty value means collect all), we should see non-zero number of files at least for cluster, container, node and hpa. The other are cluster-specific.
	If the numbers are lower than expected, you probably have issues with sending container data to Densify and need to review the rest of the log and contact Densify support. Otherwise, you can move on to the next step.
	
	Once the collected container data is sent to Densify, the pod exits.

9. Create the cron job using `cronjob.yaml`

    `kubectl create -f cronjob.yaml -n <namespace>`

The cron job will run and send data to Densify hourly. You should adjust the cron job schedule to run on the same `interval_size` you are using for data collection, as defined in the config map.
