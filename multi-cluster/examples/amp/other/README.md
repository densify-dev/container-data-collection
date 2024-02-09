This example shows you how to setup the Data Forwarder to run on a Kubernetes cluster not on AWS EKS, connect to Amazon Managed Prometheus (AMP) and send container data to Densify on an hourly basis. 

Not running on an AWS EKS cluster means we cannot make use of EKS ability to associate Kubernetes service accounts with AWS IAM roles. Therefore the Forwarder is not running under a service account, but the AWS credentials need to be provided to it.

These credentials can be specified explicitly in the config map, but this is not a good practice. Instead we opt to run a shell script, `create-user-policy-secret.sh`, which takes care of that.

## Pre-requisites

Steps 1-6 require a Linux environment with `bash` and the following utilities:

* [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html#getting-started-install-instructions)
* [jq](https://jqlang.github.io/jq/)
* base64 - pre-installed in most Linux distros as part of the `coreutils` package

The shell script needs to be edited as follows:

1. Fill in `AWS_REGION` (mandatory)

2. To create a new AWS IAM user and create an access key / secret key pair for this user, fill in `AWS_USER` and `AWS_PASSWORD` (the latter between single quotes), and leave `AWS_ACCESS_KEY` and `AWS_SECRET_KEY` blank

3. To use instead an existing AWS IAM user and its active access key / secret key pair, fill in `AWS_USER`, `AWS_ACCESS_KEY` and `AWS_SECRET_KEY` and leave `AWS_PASSWORD` blank

4. To create a new AWS IAM policy which grants access rights to issue queries to AMP leave `AMP_QUERY_POLICY_ARN` blank

5. To use instead an existing such policy, fill in `AMP_QUERY_POLICY_ARN`

6. Run the script. This will:

* create the user if required
* create the policy if required
* attach the above policy to user
* create the AWS user config and credentials files based on the AWS region and the user's credentials
* create a yaml file for a Kubernetes secret with the AWS config and credentials files

7. Now, create the AWS secret in Kubernetes.
    
    `kubectl create -f aws-secret.yaml -n <namespace>`

Now edit the `configmap.yaml` file, then create the config map to pass the settings to `config.yaml`. To test the Data Forwarder setup, create a pod to ensure that data is sent to Densify before enabling the cronjob to run data collection every hour.

8. Modify `configmap.yaml` to point to your Densify instance and to your AMP workspace.

9. Create the config map in Kubernetes
    
    `kubectl create -f configmap.yaml -n <namespace>`
	
10. Create the pod using `pod.yaml`
    
    `kubectl create -f pod.yaml -n <namespace>`
	
11. Review the log for the container.
	
	`kubectl logs densify -n <namespace>`
	
	You should see lines similar to the following, near the end of the log:

	> {"level":"info","pkg":"default","time":1704496040349770,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-1, zipping cluster-1.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; total - 54 files"}

	> {"level":"info","pkg":"default","time":1704496040449763,"caller":"src/container/forwarderv2/files.go:98","goid":1,"message":"cluster : cluster-2, zipping cluster-2.zip, contents: cluster - 21 files; container - 16 files; node - 17 files; node_group - 0 files; hpa - 0 files; rq - 0 files; crq - 0 files; total - 54 files"}

	> {"level":"info","pkg":"default","file":"data/cluster-1.zip","time":1704496040616014,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"file uploaded successfully"}

	> {"level":"info","pkg":"default","file":"data/cluster-2.zip","time":1704496040666046,"caller":"src/container/forwarderv2/main.go:57","goid":1,"message":"file uploaded successfully"}

	The exact number of files - under each subfolder and total - depends on the Data Forwarder `include_list` configuration, kube-state-metrics configuration and what is defined/running in the Kubernetes cluster we collect data for. If we use the default `include_list` configuration (empty value means collect all), we should see non-zero number of files at least for cluster, container, node and hpa. The other are cluster-specific.
	If the numbers are lower than expected, you probably have issues with sending container data to Densify and need to review the rest of the log and contact Densify support. Otherwise, you can move on to the next step.
	
	Once the collected container data is sent to Densify, the pod exits.
		
12. Cleanup

    `kubectl delete -f pod.yaml -n <namespace>`

13. Troubleshooting

	In case of errors in the logs and/or small amount of files, we can check the AWS secret, AWS IAM role AND the configuration this way.

	Edit `awscurl.yaml` - in the values of the two env vars replace `<AWS region>` (twice) and `<AMP workspace ID>` (once) with their values. Save and run:

    `kubectl create -f awscurl.yaml -n <namespace>`

	Now run this until you see that the `awscurl` pod is running:

    `kubectl get pod awscurl -n <namespace>`

	Now shell into the pod:

    `kubectl exec -it awscurl -n <namespace> -- sh`

	In the shell, run the following command (no need to edit it):
	
    `awscurl -X POST --region ${REGION} --service aps "${AMP_QUERY_ENDPOINT}" -d 'query=up' --header 'Content-Type: application/x-www-form-urlencoded'`
	
	If `awscurl` reports an error, there's an issue with the AWS secret or AWS IAM role. If the call succeeds but the result is empty, it means we are connecting to an empty AMP workspace so something is wrong with our setup. If we get a non-empty result this part is OK.

	Next, we want to test the cluster identifiers in the config map by running a PromQL query which should return data. For each `<label name>: <label value>` pair in each cluster's identifiers, replace these in the following command and run it:

    `awscurl -X POST --region ${REGION} --service aps "${AMP_QUERY_ENDPOINT}" -d 'query=kube_node_info{<label name>="<label value>"}' --header 'Content-Type: application/x-www-form-urlencoded'`

	Each one of these runs should return a non-empty result.

	Now exit the shell using

	`exit`

	and clean up the `awscurl` pod running

    `kubectl delete -f awscurl.yaml -n <namespace>`

	If you have found any errors in either the AWS secret or the config map, fix those and return to the right step in the procedure.

14. Create the cronjob using `cronjob.yaml`
    
    `kubectl create -f cronjob.yaml -n <namespace>`
	
	The cronjob runs and sends the collected container data to Densify hourly. You need to schedule the pod to run at the same `collection.interval_size` that is configured for data collection, as defined in the config map.
