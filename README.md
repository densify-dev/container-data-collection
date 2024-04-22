# Densify Container Optimization Data Forwarder

<img src="https://www.densify.com/wp-content/uploads/densify.png" width="300">

The Densify Container Optimization Data Forwarder is the container that collects data from Kubernetes using the Prometheus API and forwards that data to Densify. Densify then analyzes your Kubernetes clusters and provides sizing recommendations. 

- [Requirements](./requirements.md)
- [Single Kubernetes Cluster Setup](#single-kubernetes-cluster)
- [Multiple Kubernetes Clusters Setup](#multiple-kubernetes-clusters)
- [Documentation](./docs)
- [Docker Images](#docker-images)
- [License](#license)

## Cluster Setup

Densify supports two use-cases for version 4 or higher of the Forwarder.

### [Single Kubernetes Cluster](./single-cluster)

In this configuration, data is collected from the Kubernetes cluster where your workloads, the Densify Forwarder and Prometheus are all running.

#### [Single Cluster Config](./single-cluster/config)

#### Single Cluster Examples

- [Kubernetes with Prometheus](./single-cluster/examples/standard)
- [Kubernetes with Authenticated Prometheus, typical case for OpenShift](./single-cluster/examples/bearer-openshift)

### [Multiple Kubernetes Clusters](./multi-cluster)

In this configuration, data is collected from multiple Kubernetes clusters monitored by an observability platform. The Densify Forwarder can run anywhere (provided it can access that observability platform).

#### [Multi-Cluster Config](./multi-cluster/config)

#### Multi-Cluster Examples

- [Kubernetes with observability platform using basic authentication](./multi-cluster/examples/basic)
- [Kubernetes with Amazon Managed Prometheus (AMP)](./multi-cluster/examples/amp)
- [Azure Monitor managed service for Prometheus (AzMP)](./multi-cluster/examples/azmp/)

## Docker images

The Docker image is available on [Docker hub](https://hub.docker.com/r/densify/container-optimization-data-forwarder/tags). Pull it using `docker pull densify/container-optimization-data-forwarder:4-beta`.

## License

Apache 2 Licensed. See [LICENSE](./LICENSE) for full details.
