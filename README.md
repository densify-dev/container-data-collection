# Densify Container Optimization Data Forwarder

<img src="https://www.densify.com/wp-content/uploads/densify.png" width="300">

The Densify Container Optimization Data Forwarder is the container that collects data from Kubernetes using the Prometheus API and forwards that data to Densify. Densify then analyzes your Kubernetes clusters and provides sizing recommendations. 

- [Requirements](#requirements)
- [Single Kubernetes Cluster Setup](#single-kubernetes-cluster)
- [Multiple Kubernetes Clusters Setup](#multiple-kubernetes-clusters)
- [Documentation](#documentation)
- [Docker Images](#docker-images)
- [License](#license)

## Requirements

- Densify account, which is provided with a [Densify subscription or through a free trial](https://www.densify.com/service/signup)
- [Kubernetes](https://kubernetes.io/) or [OpenShift](https://www.redhat.com/en/technologies/cloud-computing/openshift) cluster
- [Prometheus or observability platform](#observability-platform-requirements)
- [Kube-state-metrics](https://github.com/kubernetes/kube-state-metrics)
- [cAdvisor](https://github.com/google/cadvisor), typically running within the kubelet
- [Node exporter](https://github.com/prometheus/node_exporter) is required for node analysis and recommendations
- [OpenShift-state-metrics](https://github.com/openshift/openshift-state-metrics)

### Observability Platform Requirements

The data is collected using the [Prometheus API](https://prometheus.io/docs/prometheus/latest/querying/api/). This requires:

- either [Prometheus, itself](https://prometheus.io/), typically deployed within the Kubernetes cluster being monitored, and where the Densify Forwarder is running.
- or an OSS/commercial observability platform which supports the Prometheus API and resides elsewhere.

Prometheus or the observability platform need to support the Prometheus API and one of the following supported authentication mechanisms:

- Unauthenticated access - applicable only for in-cluster Prometheus;
- HTTP basic authentication - supported by Prometheus and required by some commercial observability platforms, e.g. [Grafana Cloud](https://grafana.com/docs/grafana-cloud/cost-management-and-billing/analyze-costs/metrics-costs/prometheus-metrics-costs/usage-analysis-api/);
- Bearer token - required by [OpenShift Monitoring](https://access.redhat.com/documentation/en-us/openshift_container_platform/4.14/html/monitoring/accessing-third-party-monitoring-apis) and some commercial observability platforms, e.g. [Azure Monitor](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql);
- AWS SigV4 - required by [Amazon Managed Service for Prometheus](https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-secure-querying.html).

## Cluster Setup

Densify supports two use-cases for version 4 or higher of the Forwarder.

### [Single Kubernetes Cluster](single-cluster)

In this configuration, data is collected from the Kubernetes cluster where your workloads, the Densify Forwarder and Prometheus are all running.

#### [Config](single-cluster/config/README.md)

#### Examples

* [Kubernetes with Prometheus](single-cluster/examples/standard)
* [Kubernetes with Authenticated Prometheus, typical case for OpenShift](single-cluster/examples/bearer-openshift)

### [Multiple Kubernetes Clusters](multi-cluster)

In this configuration, data is collected from multiple Kubernetes clusters monitored by an observability platform. The Densify Forwarder can run anywhere (provided it can access that observaibility platform).

#### [Config](multi-cluster/config/README.md)

#### Examples

* [Kubernetes with observability platfrom using basic authentication](multi-cluster/examples/basic)

## [Documentation](docs)

## Docker images

The Docker image is available on [Docker hub](https://hub.docker.com/r/densify/container-optimization-data-forwarder/tags). Pull it using `docker pull densify/container-optimization-data-forwarder:4-beta`.

## License

Apache 2 Licensed. See [LICENSE](LICENSE) for full details.
