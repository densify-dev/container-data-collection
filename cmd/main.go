package main

import (
	cconf "github.com/densify-dev/container-config/config"
	"github.com/densify-dev/container-data-collection/internal/cluster"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/container"
	"github.com/densify-dev/container-data-collection/internal/crq"
	"github.com/densify-dev/container-data-collection/internal/node"
	"github.com/densify-dev/container-data-collection/internal/nodegroup"
	"github.com/densify-dev/container-data-collection/internal/rq"
)

func main() {
	var err error
	common.Params, err = cconf.ReadConfig()
	if err != nil {
		common.FatalError(err, "Failed to read configuration:")
	}
	common.SetCurrentTime()
	if err = common.RegisterClusterFilters(common.Params.Clusters); err != nil {
		common.FatalError(err, "Failed to register cluster filters:")
	}
	if err = common.MkdirAll(); err != nil {
		common.FatalError(err, "Failed to create directories:")
	}
	common.InitLogs()
	common.LogAll(1, common.Info, "Container data collection version %s", common.Version)
	if upCount := common.CheckPrometheusUp(); upCount == 0 {
		common.LogAll(1, common.Warn, "Prometheus server is up but reports no `up` metrics with value 1 for any scrape config, please verify it is actually scraping / collecting data")
	} else {
		common.LogAll(1, common.Info, "Prometheus server is up and reports `up` metrics with value 1 for scrape config(s)")
	}
	ver, verFound := common.GetPrometheusVersion()
	var logVerPrefix string
	if verFound {
		logVerPrefix = "Detected "
	}
	common.LogAll(1, common.Info, "%sPrometheus version %s", logVerPrefix, ver)
	if includes(common.ContainerEntityKind) {
		container.Metrics()
	} else {
		common.LogAll(1, common.Info, "Skipping container data collection")
	}
	if includes(common.NodeEntityKind) {
		node.Metrics()
	} else {
		common.LogAll(1, common.Info, "Skipping node data collection")
	}
	if includes(common.NodeGroupInclude) {
		nodegroup.Metrics()
	} else {
		common.LogAll(1, common.Info, "Skipping node group data collection")
	}
	if includes(common.ClusterEntityKind) {
		cluster.Metrics()
	} else {
		common.LogAll(1, common.Info, "Skipping cluster data collection")
	}
	if includes(common.Quota) {
		crq.Metrics()
		rq.Metrics()
	} else {
		common.LogAll(1, common.Info, "Skipping quota data collection")
	}
}

func includes(entityKind string) bool {
	return entityKind == common.ClusterEntityKind ||
		len(common.Params.Collection.Include) == 0 ||
		common.Params.Collection.Include[entityKind]
}
