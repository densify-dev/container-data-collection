# Changelog

Version 4 introduces significant updates of Densify Container Data Collection. It resides in a new [Github repository](https://github.com/densify-dev/container-data-collection), which replaces the [versions 1-3 repository](https://github.com/densify-dev/Container-Optimization-Data-Forwarder), to be deprecated on June 30th, 2024.

This file documents the changes in both repositories. Those in this repository starts with the `4.0.0-beta` release.

## 4.0.0

* Documentation and examples updates
* Full list of changes and bug fixes - see [here](./v4.md)
* Corresponds to image tag `densify/container-optimization-data-forwarder:4`

## 4.0.0-beta

* Multiple k8s cluster support
* External observability platform support
* Corresponds to image tag `densify/container-optimization-data-forwarder:4-beta`

## 3.0.0

* Usage of a new endpoint to support TSDB
* Corresponds to image tag `densify/container-optimization-data-forwarder:3`

## 2.3.0

* Add support for resource quotas and cluster resource quotas
* Add support for changing label used for building node groups
* Add additional network, disk metrics to cluster entity
* Bug fixes for logging

## 2.2.0

* Add support for node groups
* Use default logger
* Add node info metric
* Bug fix for network and disk rates on nodes

## 2.1.2

* Refactor and update to support new and older versions of Kubernetes for number of metrics

## 2.1.1

* Fix node memory metric name

## 2.1.0

* Added additional workloads

## 2.0.2

* Update to permissions of data directory for when running as other users

## 2.0.1

* Add support for passing parameters via Environment variables
* Add cluster level metrics
* Update log handling
* Fixed bug in queries where could have error grouping rows based on duplicates
* Fixed bug in queries for counter metrics that needed to be rated
* Cleaned up unused files in the project

## 2.0.0

* Converted project from Python and Java to Go
* Added Node level metrics
* Added Deployment and HPA support
* Created multistage Docker build
* Added Cronjob example

## 1.0.2

* Add support for specifying the cluster to use for cases when Prometheus address names were identical across environments

## 1.0.1

* Changed to use Alpine openJDK base image

## 1.0.0

* Initial release of container data collection from Prometheus
