# Requirements

- Densify account, which is provided with a [Densify subscription or through a free trial](https://www.densify.com/service/signup)
- [Kubernetes](https://kubernetes.io/) or [OpenShift](https://www.redhat.com/en/technologies/cloud-computing/openshift) cluster or clusters
- [Egress Requirements](./egress-requirements.md) for your kubernetes / OpenShift cluster
- [Prometheus or observability platform data source](#data-source)
- [Kube-state-metrics](https://github.com/kubernetes/kube-state-metrics), see specific requirements [here](#kube-state-metrics-requirements)
- [cAdvisor](https://github.com/google/cadvisor), typically running within the kubelet
- [Node exporter](https://github.com/prometheus/node_exporter) is required for node analysis and recommendations
- [OpenShift-state-metrics](https://github.com/openshift/openshift-state-metrics) is required for OpenShift clusters only
- [DCGM Exporter](https://docs.nvidia.com/datacenter/cloud-native/gpu-telemetry/latest/dcgm-exporter.html), which code is [here](https://github.com/NVIDIA/dcgm-exporter), is required for any Kubernetes cluster which uses Nvidia GPUs. For full Nvidia GPU data it is recommended to install the [Nvidia GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/index.html) in the cluster (the operator installs the DCGM exporter itself)
- The list of required metrics of the various exporters is described [here](./docs/README.md)

## Data Source

The data is collected using the [Prometheus API](https://prometheus.io/docs/prometheus/latest/querying/api/). This requires:

- either [Prometheus, itself](https://prometheus.io/), typically deployed within the Kubernetes cluster being monitored, and where the Densify Forwarder is running
- or an OSS/commercial observability platform which supports the Prometheus API (and resides elsewhere)

### Data Retention

The Forwarder typically runs as a cronjob some time after the hour and collects data for the previous hour. This means that the absolute minimum data retention required is 2h, though for contingency and data recovery scenarios it is recommended to set it on a few days. As commercial observability platforms typically have much longer data retention, this applies mainly for self-hosted Prometheus servers or OSS observability platforms.

### Supported Commercial Observability Platforms

- [Amazon Managed Service for Prometheus](https://docs.aws.amazon.com/prometheus/latest/userguide/index.html)
- [Azure Monitor managed service for Prometheus](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-overview)
- [Grafana Cloud](https://grafana.com/products/cloud/)

### Supported Self-Hosted OSS Solutions

- [Thanos](https://thanos.io/)
- [Cortex](https://cortexmetrics.io/)
- [Grafana Mimir](https://grafana.com/oss/mimir/)
- [Prometheus Federation](https://prometheus.io/docs/prometheus/latest/federation/)

### Authentication

Prometheus or the observability platform need to support the Prometheus API and one of the following supported authentication mechanisms:

- Unauthenticated access - applicable only for in-cluster Prometheus;
- HTTP basic authentication - supported by Prometheus and required by some commercial observability platforms, e.g. [Grafana Cloud](https://grafana.com/docs/grafana-cloud/cost-management-and-billing/analyze-costs/metrics-costs/prometheus-metrics-costs/usage-analysis-api/);
- Bearer token - required by [OpenShift Monitoring](https://access.redhat.com/documentation/en-us/openshift_container_platform/4.14/html/monitoring/accessing-third-party-monitoring-apis) and some commercial observability platforms, e.g. [Azure Monitor managed service for Prometheus](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql);
- AWS SigV4 - required by [Amazon Managed Service for Prometheus](https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-secure-querying.html).

## Data Integrity

### In-cluster Prometheus

In this use-case it is assumed that **all** data in Prometheus (of the relevant exporters) comes from the kubernetes / OpenShift cluster we are collecting data for.

### Observability Platform

In this use-case the observability platform typically collects data from multiple clusters and/or other sources. It is assumed that the data (of **all relevant exporters**) is identifiable by a **unique set of labels (names and values)** for each kubernetes / OpenShift cluster we are collecting data for. The set of labels is typically achieved using `global.external_labels` in the [configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file) of the source Prometheus server / OTel collector feeding the data into the observability platform.

### Exporters

The following applies for both in-cluster Prometheus and observability platform, per cluster:

- kube-state-metrics - one and only one instance of kube-state-metrics should be scraped;
- openshift-state-metrics (OpenShift clusters only) - one and only one instance of openshift-state-metrics should be scraped;
- cAdvisor is typically running within the kubelet; cAdvisors of **all** cluster nodes' kubelets, and **only** of the cluster nodes' kubelets, should be scraped;
- Node exporter of **all** cluster nodes, and **only** of the cluster nodes, should be scraped; scraping node exporters from virtual machines and/or cloud instances which are not cluster nodes will cause data integrity issues;
- DCGM exporter of **all** cluster nodes equipped with Nvidia GPUs, and **only** of these cluster nodes, should be scraped

## Kube-state-metrics Requirements

Densify requires kube-state-metrics `v1.5.0` or later. When using `v2.x` or higher, some additional considerations are required. The default settings of kube-state-metric `v2.x` no longer include the collection of Kubernetes object labels nor annotations.

Collection of Kubernetes node labels is essential for the `node group` data collection feature. To enable this feature with kube-state-metrics `v2.x` or higher, add the following command-line argument to the kube-state-metrics container:

```shell
["--metric-labels-allowlist=nodes=[*]"]
```

In addition to node labels, Densify attempts to collect the following labels and annotations, which can be further used as sort/filter criteria and to generate custom reports. To collect this data with kube-state-metrics `v2.x` or higher, add the following command-line arguments to the kube-state-metrics container:

```shell
["--metric-labels-allowlist=nodes=[*],namespaces=[*],pods=[*],deployments=[*],replicasets=[*],daemonsets=[*],statefulsets=[*],jobs=[*],cronjobs=[*],horizontalpodautoscalers=[*]", "--metric-annotations-allowlist=namespaces=[*]"]
```
