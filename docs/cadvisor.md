# Kubernetes, kubelet & cAdvisor

## Kubernetes & Kubelet Metrics

| Prometheus Metric Name                  | Description/Usage                           | C[^1] | N[^2]              | NG[^3] | Cl[^4]             | RQ[^5] | CRQ[^6] |
| --------------------------------------- | ------------------------------------------- | ------| ------------------ | ------ | ------------------ | ------ | ------- |
| kubernetes_build_info                   | API servers & kubelet build information     |       | :white_check_mark: |        | :white_check_mark: |        |         |

## cAdvisor Metrics

| Prometheus Metric Name                  | Description/Usage                           | C[^1]              | N[^2] | NG[^3] | Cl[^4] | RQ[^5] | CRQ[^6] |
| --------------------------------------- | ------------------------------------------- | ------------------ | ----- | ------ | ------ | ------ | ------- |
| container_cpu_cfs_periods_<br/>total    | Container total CPU periods                 | :white_check_mark: |       |        |        |        |         |
| container_cpu_cfs_throttled_<br/>periods_total  | Container throttled CPU periods             | :white_check_mark: |       |        |        |        |         |
| container_cpu_cfs_throttled_<br/>seconds_total  | Container throttled CPU seconds             | :white_check_mark: |       |        |        |        |         |
| container_cpu_usage_seconds_<br/>total  | Container CPU utilization (in core-seconds) | :white_check_mark: |       |        |        |        |         |
| container_fs_usage_bytes                | Container raw disk utilization              | :white_check_mark: |       |        |        |        |         |
| container_memory_rss                    | Container actual memory utilization         | :white_check_mark: |       |        |        |        |         |
| container_memory_usage_bytes            | Container raw memory utilization            | :white_check_mark: |       |        |        |        |         |
| container_memory_working_set_<br/>bytes | Container memory working set                | :white_check_mark: |       |        |        |        |         |
| container_oom_events_total              | Container number of OOM Kills               | :white_check_mark: |       |        |        |        |         |
| container_spec_memory_limit_<br/>bytes  | Container memory limit                      | :white_check_mark: |       |        |        |        |         |

[^1]: Container Metrics
[^2]: Node Metrics
[^3]: Node Group Metrics
[^4]: Cluster Metrics
[^5]: Resource Quota Metrics
[^6]: Cluster Resource Quota Metrics
