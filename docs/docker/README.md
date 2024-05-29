# Densify Container Optimization Data Forwarder

<img src="https://www.densify.com/wp-content/uploads/densify.png" width="300">

The Densify Container Optimization Data Forwarder collects data from Kubernetes using the Prometheus API and forwards that data to Densify. Densify then analyzes your Kubernetes clusters and provides sizing recommendations.

## Quick Reference

Maintained by: [Densify](https://github.com/densify-dev/container-data-collection)

Please refer to Github for [prerequisites](https://github.com/densify-dev/container-data-collection/blob/main/requirements.md) and detailed [usage examples](https://github.com/densify-dev/container-data-collection/blob/main/README.md#cluster-setup).

## Supported Tags

- `4`; only use this tag as it will be updated with minor and patch releases

Updates will be backwards-compatible.

## Deprecated Tags

All of the following tags have been deprecated and will no longer be supported after 2024-06-30. In particular, the `latest` tag should not be used.

- `3`
- All `3.X` tags
- All `alpine-3.X` tags
- All `release-2.X` tags
- All `release-1.X` tags
- `latest`

## Base Image

- [Alpine](https://hub.docker.com/_/alpine)

## License

Apache 2 Licensed. See [LICENSE](https://github.com/densify-dev/container-data-collection/blob/main/LICENSE) for full details.
