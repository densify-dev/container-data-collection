# Densify Container Data Forwarder

<img src="https://www.densify.com/wp-content/uploads/densify.png" width="300">

This repository contains the source code for the Densify Container Data Forwarder image. The forwarder collects data from Kubernetes using the Prometheus API and forwards that data to Densify for analysis and optimization recommendations.

## Deployment

This image is deployed as part of the [kubex-automation-stack Helm chart](https://github.com/densify-dev/kubex-automation-stack), which bundles all required dependencies including:
- Prometheus
- cAdvisor  
- kube-state-metrics
- node-exporter
- DCGM exporter (for GPU monitoring)

For installation instructions, configuration details, and requirements, please refer to the [kubex-automation-stack documentation](https://github.com/densify-dev/kubex-automation-stack).

## Docker Images

The Docker image is available on [Docker Hub](https://hub.docker.com/r/densify/container-optimization-data-forwarder/tags). Pull it using:
```bash
docker pull densify/container-optimization-data-forwarder:4
```

## Development

This is a Go-based application that queries Prometheus metrics and forwards them to Densify's platform.

## License

Apache 2 Licensed. See [LICENSE](./LICENSE) for full details.
