# Amazon Managed Service for Prometheus

This example shows you how to setup the Data Forwarder to connect to [Amazon Managed Service for Prometheus](https://docs.aws.amazon.com/prometheus/latest/userguide/index.html) and send container data to Densify on an hourly basis. It is split into two separate deployments of the Forwarder:

* On an [AWS EKS Kubernetes Cluster](./eks)
* On any [other Kubernetes Cluster](./other)
