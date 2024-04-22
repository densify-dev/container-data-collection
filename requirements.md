# Requirements

- Densify account, which is provided with a [Densify subscription or through a free trial](https://www.densify.com/service/signup)
- [Kubernetes](https://kubernetes.io/) or [OpenShift](https://www.redhat.com/en/technologies/cloud-computing/openshift) cluster or clusters
- [Prometheus or observability platform data source](#data-source)
- [Kube-state-metrics](https://github.com/kubernetes/kube-state-metrics), see specific requirements [here](#kube-state-metrics-requirements)
- [cAdvisor](https://github.com/google/cadvisor), typically running within the kubelet
- [Node exporter](https://github.com/prometheus/node_exporter) is required for node analysis and recommendations
- [OpenShift-state-metrics](https://github.com/openshift/openshift-state-metrics)
- The list of required metrics of the various exporters is described [here](./docs/README.md)

## Data Source

The data is collected using the [Prometheus API](https://prometheus.io/docs/prometheus/latest/querying/api/). This requires:

- either [Prometheus, itself](https://prometheus.io/), typically deployed within the Kubernetes cluster being monitored, and where the Densify Forwarder is running
- or an OSS/commercial observability platform which supports the Prometheus API (and resides elsewhere)

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
