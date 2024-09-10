# Prometheus Metrics

The following documents list the Prometheus metrics collected by Densify's container data collection. For convenience purposes, these are split according to the Prometheus exporter producing the metrics:

- [kube-state-metrics](./kube-state-metrics.md)
- [Kubernetes, kubelet & cAdvisor](./cadvisor.md)
- [Node Exporter](./node-exporter.md)
- [openshift-state-metrics](./openshift-state-metrics.md)

Each document also shows how these Prometheus metrics are used by Densify:

- Container Metrics
- Node Metrics
- Node Group Metrics
- Cluster Metrics
- Resource Quota (RQ) Metrics
- Cluster Resource Quota (CRQ) Metrics

Densify container data collection supports popular Prometheus relabel configs, such as those in:

- [Prometheus community helm charts](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus)
- [Prometheus Operator](https://github.com/prometheus-operator/kube-prometheus)
