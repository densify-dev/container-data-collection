package node

import (
	"github.com/densify-dev/container-data-collection/internal/common"
	"strings"
)

const (
	// GKE consts
	cloud       = "cloud"
	google      = "google"
	gke         = "gke"
	accelerator = "accelerator"
	shared      = "shared"
	clients     = "clients"
	per         = "per"
	// Nvidia consts
	nvidiaPciVendorId = "pci_10de"
	present           = "present"
	product           = "product"
	mig               = "mig"
	capable           = "capable"
	vgpu              = "v" + common.Gpu
	Model             = "model" // model is already imported in this package
	// shared consts
	mps      = "mps"
	sharing  = "sharing"
	strategy = "strategy"
)

var (
	labelNvidiaComponents    = []string{common.Label, common.Nvidia, common.Com}
	labelNvidiaGpuComponents = appendGpu(labelNvidiaComponents)
	labelGkeComponents       = []string{common.Label, cloud, google, common.Com, gke}
	labelGkeGpuComponents    = appendGpu(labelGkeComponents)
	prefixComponents         = map[string]map[bool][]string{
		common.Nvidia: {
			false: labelNvidiaComponents,
			true:  labelNvidiaGpuComponents,
		},
		gke: {
			false: labelGkeComponents,
			true:  labelGkeGpuComponents,
		},
	}
	nvidiaLabelPrefix             = getLabelName(common.Nvidia, false)
	gkeLabelPrefix                = getLabelName(gke, false)
	ModelName                     = common.DromedaryCase(Model, common.Name)
	memTotal                      = common.DromedaryCase(common.Mem, common.Total)
	candidateMissingGpuAttributes = []string{Model, memTotal}
	missingGpuAttributes          = make(map[string]bool)
)

func isGpuLabel(key string) bool {
	return strings.HasPrefix(key, nvidiaLabelPrefix) ||
		(strings.HasPrefix(key, gkeLabelPrefix) && (strings.Contains(key, common.Gpu) || strings.Contains(key, accelerator))) ||
		strings.Contains(key, nvidiaPciVendorId)
}

type gkeGpuData struct {
	modelName        string
	sharingStrategy  string
	maxSharedClients int
}

func (g *gkeGpuData) hasData() bool {
	return g.modelName != common.Empty || g.sharingStrategy != common.Empty || g.maxSharedClients > 0
}

func applyGpuLabels(cluster string, n *node, gpuLabels map[string]string) {
	ggd := &gkeGpuData{}
	common.SetLabelValue(cluster, &ggd.modelName, gpuLabels, getLabelName(gke, false, accelerator))
	common.SetLabelValue(cluster, &ggd.sharingStrategy, gpuLabels, getLabelName(gke, true, sharing, strategy))
	common.SetLabelValue(cluster, &ggd.maxSharedClients, gpuLabels, getLabelName(gke, false, common.Max, shared, clients, per, common.Gpu))
	var nvidiaGpuPresent bool
	common.SetLabelValue(cluster, &nvidiaGpuPresent, gpuLabels, getLabelName(common.Nvidia, true, present))
	if !nvidiaGpuPresent {
		if ggd.hasData() {
			n.gpuVendor = common.Nvidia
			n.gpuModel = ggd.modelName
			n.gpuSharingStrategy = ggd.sharingStrategy
			if n.gpuSharingStrategy == mps {
				n.gpuMpsCapable = true
			}
			n.gpuReplicas = ggd.maxSharedClients
		}
		return
	}
	n.gpuVendor = common.Nvidia
	common.SetLabelValue(cluster, &n.gpuModel, gpuLabels, getLabelName(common.Nvidia, true, product))
	common.SetLabelValue(cluster, &n.gpuTotal, gpuLabels, getLabelName(common.Nvidia, true, common.Count))
	common.SetLabelValue(cluster, &n.gpuReplicas, gpuLabels, getLabelName(common.Nvidia, true, common.Replicas))
	common.SetLabelValue(cluster, &n.gpuMemTotal, gpuLabels, getLabelName(common.Nvidia, true, common.Memory))
	common.SetLabelValue(cluster, &n.gpuSharingStrategy, gpuLabels, getLabelName(common.Nvidia, true, sharing, strategy))
	common.SetLabelValue(cluster, &n.gpuMigCapable, gpuLabels, getLabelName(common.Nvidia, false, mig, capable))
	if n.gpuMigCapable {
		common.SetLabelValue(cluster, &n.gpuMigStrategy, gpuLabels, getLabelName(common.Nvidia, false, mig, strategy))
	}
	common.SetLabelValue(cluster, &n.gpuMpsCapable, gpuLabels, getLabelName(common.Nvidia, false, mps, capable))
	common.SetLabelValue(cluster, &n.gpuVgpuPresent, gpuLabels, getLabelName(common.Nvidia, false, vgpu, present))
}

func appendGpu(s []string) []string {
	return append(s, common.Gpu)
}

func getLabelName(labelProvider string, includeGpu bool, elements ...string) string {
	return common.SnakeCase(append(prefixComponents[labelProvider][includeGpu], elements...)...)
}
