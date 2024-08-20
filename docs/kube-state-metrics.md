# kube-state-metrics

## Metrics

| Prometheus Metric Name                                           | Description/Usage                    | C[^1]              | N[^2]              | NG[^3]             | Cl[^4]             | RQ[^5]             | CRQ[^6] |
| ---------------------------------------------------------------- | ------------------------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------- |
| kube_cronjob_created                                             | CronJob creation time                | :white_check_mark: |                    |                    |                    |                    |         |
| kube_cronjob_info                                                | CronJob information                  | :white_check_mark: |                    |                    |                    |                    |         |
| kube_cronjob_labels                                              | CronJob labels                       | :white_check_mark: |                    |                    |                    |                    |         |
| kube_cronjob_next_schedule_<br/>time                             | CronJob next schedule time           | :white_check_mark: |                    |                    |                    |                    |         |
| kube_cronjob_status_active                                       | CronJob status active                | :white_check_mark: |                    |                    |                    |                    |         |
| kube_cronjob_status_last_<br/>schedule_time                      | CronJob last schedule time           | :white_check_mark: |                    |                    |                    |                    |         |
| kube_daemonset_created                                           | DaemonSet creation time              | :white_check_mark: |                    |                    |                    |                    |         |
| kube_daemonset_labels                                            | DaemonSet labels                     | :white_check_mark: |                    |                    |                    |                    |         |
| kube_daemonset_status_number_<br/>available                      | DaemonSet current size               | :white_check_mark: |                    |                    |                    |                    |         |
| kube_deployment_created                                          | Deployment creation time             | :white_check_mark: |                    |                    |                    |                    |         |
| kube_deployment_labels                                           | Deployment labels                    | :white_check_mark: |                    |                    |                    |                    |         |
| kube_deployment_metadata_<br/>generation                         | Deployment metadata generation       | :white_check_mark: |                    |                    |                    |                    |         |
| kube_deployment_spec_strategy_<br/>rollingupdate_max_surge       | Deployment max surge                 | :white_check_mark: |                    |                    |                    |                    |         |
| kube_deployment_spec_strategy_<br/>rollingupdate_max_unavailable | Deployment max unavailable           | :white_check_mark: |                    |                    |                    |                    |         |
| kube_horizontalpodautoscaler_<br/>info                           | HPA information                      | :white_check_mark: |                    |                    |                    |                    |         |
| kube_horizontalpodautoscaler_<br/>labels                         | HPA labels                           | :white_check_mark: |                    |                    |                    |                    |         |
| kube_horizontalpodautoscaler_<br/>spec_max_replicas              | HPA max replicas                     | :white_check_mark: |                    |                    |                    |                    |         |
| kube_horizontalpodautoscaler_<br/>spec_min_replicas              | HPA min replicas                     | :white_check_mark: |                    |                    |                    |                    |         |
| kube_horizontalpodautoscaler_<br/>status_condition               | HPA scaling limited                  | :white_check_mark: |                    |                    |                    |                    |         |
| kube_horizontalpodautoscaler_<br/>status_current_replicas        | HPA current replicas                 | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_created                                                 | Job creation time                    | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_info                                                    | Job information                      | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_labels                                                  | Job labels                           | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_owner                                                   | Job owner                            | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_spec_completions                                        | Job spec completions                 | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_spec_parallelism                                        | Job spec parallelism                 | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_status_completion_<br/>time                             | Job status completion time           | :white_check_mark: |                    |                    |                    |                    |         |
| kube_job_status_start_time                                       | Job status start time                | :white_check_mark: |                    |                    |                    |                    |         |
| kube_namespace_annotations                                       | Namespace annotations                | :white_check_mark: |                    |                    |                    |                    |         |
| kube_namespace_labels                                            | Namespace labels                     | :white_check_mark: |                    |                    |                    |                    |         |
| kube_node_info                                                   | Node info                            |                    | :white_check_mark: |                    |                    |                    |         |
| kube_node_labels                                                 | Node labels                          |                    | :white_check_mark: | :white_check_mark: |                    |                    |         |
| kube_node_role                                                   | Node role (label)                    |                    | :white_check_mark: |                    |                    |                    |         |
| kube_node_status_allocatable                               | Node allocatable                        |                    | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |         |
| kube_node_status_capacity                                        | Node capacity                        |                    | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |         |
| kube_pod_container_info                                          | Container information                | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_container_resource_<br/>limits                          | Container CPU limit                  | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |         |
| kube_pod_container_resource_<br/>requests                        | Container CPU requests               | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |         |
| kube_pod_container_status_<br/>last_terminated_exitcode                    | Container last exit code                   | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_container_status_<br/>last_terminated_timestamp                    | Container last exit timestamp                   | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_container_status_<br/>restarts_total                    | Container restarts                   | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_container_status_<br/>terminated                        | Container power state                | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_container_status_<br/>terminated_reason                 | Container power state                | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_created                                                 | Pod creation time                    | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_info                                                    | Pod information                      | :white_check_mark: | :white_check_mark: |                    |                    |                    |         |
| kube_pod_labels                                                  | Pod labels                           | :white_check_mark: |                    |                    |                    |                    |         |
| kube_pod_owner                                                   | Pod owner                            | :white_check_mark: |                    |                    |                    |                    |         |
| kube_replicaset_created                                          | ReplicaSet creation time             | :white_check_mark: |                    |                    |                    |                    |         |
| kube_replicaset_labels                                           | ReplicaSet labels                    | :white_check_mark: |                    |                    |                    |                    |         |
| kube_replicaset_owner                                            | ReplicaSet owner                     | :white_check_mark: |                    |                    |                    |                    |         |
| kube_replicaset_spec_replicas                                    | ReplicaSet current size              | :white_check_mark: |                    |                    |                    |                    |         |
| kube_replicationcontroller_<br/>created                          | Replication controller creation time | :white_check_mark: |                    |                    |                    |                    |         |
| kube_replicationcontroller_<br/>spec_replicas                    | Replication controller size          | :white_check_mark: |                    |                    |                    |                    |         |
| kube_resourcequota                                               | Namespace Quotas                     | :white_check_mark: |                    |                    |                    | :white_check_mark: |         |
| kube_resourcequota_created                                       | Resource Quota creation time         |                    |                    |                    |                    | :white_check_mark: |         |
| kube_statefulset_created                                         | StatefulSet creation time            | :white_check_mark: |                    |                    |                    |                    |         |
| kube_statefulset_labels                                          | StatefulSet labels                   | :white_check_mark: |                    |                    |                    |                    |         |
| kube_statefulset_replicas                                        | StatefulSet current size             | :white_check_mark: |                    |                    |                    |                    |         |

[^1]: Container Metrics
[^2]: Node Metrics
[^3]: Node Group Metrics
[^4]: Cluster Metrics
[^5]: Resource Quota Metrics
[^6]: Cluster Resource Quota Metrics

## Legacy Metrics

Both Kubernetes and kube-state-metrics have seen along the years changes, deprecation and removal of features. These resulted in changes, deprecation and removal of kube-state-metrics metrics, and replacement of those by other metrics. 

Densify's container data collection supports kube-state-metrics of version 1.5 or higher. In case your monitoring stack is running one of the older versions, it is possible that some of the metrics mentioned in [Metrics](#metrics) are absent. In this case, Densify's container data collection will collect the older metrics, which are produced by older versions of kube-state-metrics. The table below summarizes the older metrics which have been since deprecated or removed, and their newer replacement metrics.

| Removed/Deprecated Metric                         | Replaced by Metric                                   |
| ------------------------------------------------- | ---------------------------------------------------- |
| kube_hpa_labels                                   | kube_horizontalpodautoscaler_labels                  |
| kube_hpa_spec_max_replicas                        | kube_horizontalpodautoscaler_spec_max_replicas       |
| kube_hpa_spec_min_replicas                        | kube_horizontalpodautoscaler_spec_min_replicas       |
| kube_hpa_status_condition                         | kube_horizontalpodautoscaler_status_condition        |
| kube_hpa_status_current_replicas                  | kube_horizontalpodautoscaler_status_current_replicas |
| kube_node_status_allocatable_cpu_cores            | kube_node_status_allocatable                         |
| kube_node_status_allocatable_memory_bytes         | kube_node_status_allocatable                         |
| kube_node_status_allocatable_pods                 | kube_node_status_allocatable                         |
| kube_node_status_capacity_cpu_cores               | kube_node_status_capacity                            |
| kube_node_status_capacity_memory_bytes            | kube_node_status_capacity                            |
| kube_node_status_capacity_pods                    | kube_node_status_capacity                            |
| kube_pod_container_resource_limits_cpu_cores      | kube_pod_container_resource_limits                   |
| kube_pod_container_resource_limits_memory_bytes   | kube_pod_container_resource_limits                   |
| kube_pod_container_resource_requests_cpu_cores    | kube_pod_container_resource_requests                 |
| kube_pod_container_resource_requests_memory_bytes | kube_pod_container_resource_requests                 |
