# Configuring the Data Forwarder on an EKS Cluster

This example shows you how to setup the data forwarder in an EKS cluster to connect to Amazon Managed Prometheus and send container data to Densify on an hourly basis.

Using an EKS cluster allows you to use EKS's ability to associate an IAM role with a Kubernetes service account and configure your pods to use the service account. This is the preferred way to connect to AMP. See [IAM roles for service accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) for details.

## Prerequisites

You need to setup the following prerequistes before deploying the data forwarder:

1. A Linux environment with `bash` and the following utilities:

   - [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html#getting-started-install-instructions)
   - [eksctl](https://eksctl.io/installation/#for-unix)
   - [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)

   The following procedure has been tested with these versions: `aws-cli/2.15.8` and `eksctl/0.167.0`.

2. An AMP workspace. See [Managing AMP Workspaces](https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-manage-ingest-query.html).

3. Configure the collection of Prometheus metrics from each cluster to your workspace, using any of the methods described [here](https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-ingest-methods.html). Ensure that all of the metrics required by Densify are collected. See [Required Prometheus Metrics](../../../../docs).

4. Configure each cluster name, from which you are collecting data, on your Prometheus server or collector. Both the label name and the value are required for `configmap.yaml`.

5. Download the following files and scripts and save them to your workspace:
	- awscurl.yaml
	- configmap.yaml
	- create-service-account.sh
	- cronjob.yaml
	- pod.yaml

## Creating the IAM Roles

1. Follow the instructions [here](https://docs.aws.amazon.com/prometheus/latest/userguide/set-up-irsa.html#set-up-irsa-query) to set up IAM roles to query AMP workspaces for Prometheus metrics.

   **Note:** You need to edit the first two lines of the sample shell script; however, in the second line the namespace should be `my_forwarder_namespace` and not `my_prometheus_namespace`. This will be the namespace where the data forwarder will be running, rather than the namespace where Prometheus or the collector are running. These 2 lines should be:

	> CLUSTER_NAME=<my_amazon_eks_clustername>

	> SERVICE_ACCOUNT_NAMESPACE=<my_forwarder_namespace>

2. Run the shell script, i.e `createIRSA-AMPQuery.sh` and verify it completes successfully.

## Creating the Service Accounts

1. Edit the first two lines of [create-service-account.sh](./create-service-account.sh) as indicated in the note above:

	> CLUSTER_NAME=<my_amazon_eks_clustername>

	> SERVICE_ACCOUNT_NAMESPACE=<my_forwarder_namespace>

2. Run the shell script using `./create-service-account.sh` and verify it completes successfully.

## Deploying the Data Forwarder

1. Edit `configmap.yaml` to connect to your Densify instance and your AMP workspace.

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

## Troubleshooting

1. If you are seeing errors in the logs and/or not enough files, review the content of `configmap.yaml` and verify your settings are correct.

2. Use the following procedure to check the service account, the AWS IAM role and the configuration.

     a. Edit `awscurl.yaml` to set the env var `<AWS region>` (twice) and `<AMP workspace ID>` (once) with their actual values.

	 b. Save the file and run:

     `kubectl create -f awscurl.yaml -n <namespace>`

	 c. Run this command until you see the `awscurl` pod is running:

     `kubectl get pod awscurl -n <namespace>`

	 d. Open a shell in the pod:

     `kubectl exec -it awscurl -n <namespace> -- sh`

	 e. Without editing the command, run the following, in the shell:

     `awscurl -X POST --region ${REGION} --service aps "${AMP_QUERY_ENDPOINT}" -d 'query=up' --header 'Content-Type: application/x-www-form-urlencoded'`

	If `awscurl` reports an error, there is an issue with the service account or the AWS IAM role.

	If the call succeeds but the result is empty, you are connecting to an empty AMP workspace so something is wrong with your setup. If you get a non-empty result then the setup is correct.

3. Test the cluster identifiers listed in the config map by running a PromQL query which should return data.

	 a. For each each cluster's identifier `<label name>: <label value>` pair, replace the values in the following command and then run it:

     `awscurl -X POST --region ${REGION} --service aps "${AMP_QUERY_ENDPOINT}" -d 'query=kube_node_info{<label name>="<label value>"}' --header 'Content-Type: application/x-www-form-urlencoded'`

	 Each execution should return a non-empty result.

	 b. Exit the shell using:

	 `exit`

	 c. Cleanup the `awscurl` pod:

     `kubectl delete -f awscurl.yaml -n <namespace>`

4. If you find any errors in either the service account or the config map, fix the issue and then return to the right step in the procedure, above.
