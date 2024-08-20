# node-exporter

## Metrics

| Prometheus Metric Name                   | Description/Usage               | C[^1] | N[^2]              | NG[^3]             | Cl[^4]             | RQ[^5] | CRQ[^6] |
| ---------------------------------------- | ------------------------------- | ----- | ------------------ | ------------------ | ------------------ | ------ | ------- |
| node_cpu_core_throttles_<br/>total       | CPU throttles per core          |       | :white_check_mark: |                    |                    |        |         |
| node_cpu_seconds_total                   | CPU utilization in core seconds |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_disk_read_bytes_total               | Disk read bytes total           |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_disk_reads_completed_<br/>total     | Disk read operations            |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_disk_writes_completed_<br/>total    | Disk write operations           |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_disk_written_bytes_total            | Disk write bytes total          |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_memory_Buffers_bytes                | Memory temporary buffers bytes  |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_memory_Cached_bytes                 | Memory page cache bytes         |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_memory_MemFree_bytes                | Raw memory utilization          |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_memory_MemTotal_bytes               | Total memory bytes              |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_network_receive_bytes_<br/>total    | Raw net received utilization    |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_network_receive_packets_<br/>total  | Network packets received        |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_network_speed_bytes                 | Network speed                   |       | :white_check_mark: |                    |                    |        |         |
| node_network_transmit_bytes_<br/>total   | Raw net sent utilization        |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_network_transmit_packets_<br/>total | Network packets sent            |       | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| node_vmstat_oom_kill                     | Number of process OOM Kills     |       | :white_check_mark: |                    |                    |        |         |

[^1]: Container Metrics
[^2]: Node Metrics
[^3]: Node Group Metrics
[^4]: Cluster Metrics
[^5]: Resource Quota Metrics
[^6]: Cluster Resource Quota Metrics
