# dcgm-exporter

## Metrics

| Prometheus Metric Name                   | Description/Usage               | C[^1] | N[^2]              | NG[^3]             | Cl[^4]             | RQ[^5] | CRQ[^6] |
| ---------------------------------------- | ------------------------------- | ----- | ------------------ | ------------------ | ------------------ | ------ | ------- |
| DCGM_FI_DEV_FB_USED                 | Framebuffer memory free (MiB) | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| DCGM_FI_DEV_FB_FREE                 | Framebuffer memory used (MiB) | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| DCGM_FI_DEV_GPU_UTIL                | GPU utilization (%) | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| DCGM_FI_DEV_POWER_USAGE             | GPU power draw (W) | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| DCGM_FI_PROF_DRAM_ACTIVE            | Ratio of cycles the device memory interface is active sending or receiving data | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| DCGM_FI_PROF_GR_ENGINE_ACTIVE       | Ratio of time the graphics engine is active | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |
| DCGM_FI_PROF_PIPE_TENSOR_ACTIVE     | Ratio of cycles the tensor (HMMA) pipe is active | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |        |         |

[^1]: Container Metrics
[^2]: Node Metrics
[^3]: Node Group Metrics
[^4]: Cluster Metrics
[^5]: Resource Quota Metrics
[^6]: Cluster Resource Quota Metrics
