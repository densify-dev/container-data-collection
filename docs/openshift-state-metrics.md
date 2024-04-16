# openshift-state-metrics

## Metrics

| Prometheus Metric Name                              | Description/Usage                         | C[^1] | N[^2] | NG[^3] | Cl[^4] | RQ[^5] | CRQ[^6]            |
| --------------------------------------------------- | ----------------------------------------- | ----- | ----- | ------ | ------ | ------ | ------------------ |
| openshift_clusterresourcequota_<br/>created         | Cluster Resource Quota creation time      |       |       |        |        |        | :white_check_mark: |
| openshift_clusterresourcequota_<br/>labels          | Cluster Resource Quota labels             |       |       |        |        |        | :white_check_mark: |
| openshift_clusterresourcequota_<br/>namespace_usage | Namespace usage information               |       |       |        |        |        | :white_check_mark: |
| openshift_clusterresourcequota_<br/>selector        | Cluster Resource Quota information        |       |       |        |        |        | :white_check_mark: |
| openshift_clusterresourcequota_<br/>usage           | CPU/memory/pods request/limit utilization |       |       |        |        |        | :white_check_mark: |

[^1]: Container Metrics
[^2]: Node Metrics
[^3]: Node Group Metrics
[^4]: Cluster Metrics
[^5]: Resource Quota Metrics
[^6]: Cluster Resource Quota Metrics
