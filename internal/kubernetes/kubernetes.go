package kubernetes

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
	"golang.org/x/exp/slices"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	clusterKubernetesVersionsLogFormat = common.ClusterFormat + common.Space + "API servers version: %s; Nodes versions: %s"
)

func Metrics() {
	query := `kubernetes_build_info{}`
	range5Min := common.TimeRange()
	_, _ = common.CollectAndProcessMetric(query, range5Min, getVersion)
	for cluster, ckv := range clusterKubernetesVersions {
		common.LogCluster(1, common.Debug, clusterKubernetesVersionsLogFormat, cluster, true, cluster, ckv.ApiServers.String(), ckv.Nodes.String())
	}
}

type BuildInfo struct {
	GitVersion string
	Major      uint64
	Minor      uint64
}

type TypedVersions struct {
	versions     []*BuildInfo
	byGitVersion map[string]*BuildInfo
	byName       map[string]*BuildInfo
	sortOnce     sync.Once
}

func (tv *TypedVersions) String() string {
	if tv == nil {
		return common.Nil
	}
	var minVer string
	var mj, mi uint64
	if bi := tv.GetMinimum(); bi == nil {
		minVer = common.Nil
	} else {
		minVer = bi.GitVersion
		mj = bi.Major
		mi = bi.Minor
	}
	return fmt.Sprintf("%d distinct versions, %d total versions, minimum version %s (%d.%d)", len(tv.versions), len(tv.byName), minVer, mj, mi)
}

func NewTypedVersions() *TypedVersions {
	return &TypedVersions{byGitVersion: make(map[string]*BuildInfo), byName: make(map[string]*BuildInfo)}
}

func Compare(a, b *BuildInfo) (res int) {
	aNil := a == nil
	bNil := b == nil
	if aNil {
		if bNil {
			res = 0
		} else {
			res = -1
		}
	} else {
		if bNil {
			res = 1
		} else {
			if res = int(a.Major - b.Major); res == 0 {
				res = int(a.Minor - b.Minor)
			}
		}
	}
	return
}

func (tv *TypedVersions) Add(name string, version *BuildInfo) {
	if tv == nil || version == nil || version.GitVersion == common.Empty || name == common.Empty {
		return
	}
	var bi *BuildInfo
	var f bool
	if bi, f = tv.byGitVersion[version.GitVersion]; !f {
		bi = version
		tv.byGitVersion[version.GitVersion] = bi
		tv.versions = append(tv.versions, bi)
	}
	tv.byName[name] = bi
}

func (tv *TypedVersions) Get(name string) *BuildInfo {
	return tv.byName[name]
}

func (tv *TypedVersions) GetMinimum() *BuildInfo {
	return tv.getExtremum(true)
}

func (tv *TypedVersions) GetMaximum() *BuildInfo {
	return tv.getExtremum(false)
}

func (tv *TypedVersions) getExtremum(isMin bool) *BuildInfo {
	if tv == nil || len(tv.versions) == 0 {
		return nil
	}
	tv.sortOnce.Do(func() {
		slices.SortStableFunc(tv.versions, func(a, b *BuildInfo) int {
			return Compare(a, b)
		})
	})
	var idx int
	if !isMin {
		idx = len(tv.versions) - 1
	}
	return tv.versions[idx]
}

type ClusterVersions struct {
	ApiServers *TypedVersions
	Nodes      *TypedVersions
}

var clusterKubernetesVersions = make(map[string]*ClusterVersions)

func GetClusterVersion(cluster string) (s string) {
	var bi *BuildInfo
	if ckv := clusterKubernetesVersions[cluster]; ckv != nil {
		if bi = ckv.ApiServers.GetMinimum(); bi == nil {
			bi = ckv.Nodes.GetMaximum()
		}
	}
	if bi != nil {
		s = bi.GitVersion
	}
	return
}

func GetNodeVersion(cluster, node string) (s string) {
	var bi *BuildInfo
	if ckv, f := clusterKubernetesVersions[cluster]; f && ckv != nil && ckv.Nodes != nil {
		bi, f = ckv.Nodes.byName[node]
	}
	if bi != nil {
		s = bi.GitVersion
	}
	return
}

func HasMinimumVersion(cluster string, major, minor uint64) bool {
	if ckv, f := clusterKubernetesVersions[cluster]; f {
		return hasMinimum(&BuildInfo{Major: major, Minor: minor}, ckv.Nodes)
	}
	return false
}

func hasMinimum(bi *BuildInfo, tvs *TypedVersions) bool {
	return Compare(bi, tvs.GetMinimum()) <= 0
}

const (
	gitVersion = "git_version"
	major      = "major"
	minor      = "minor"
	kubelet    = "kubelet"
	apiServers = "apiservers"
)

var nodes = common.Plural(common.Node)

var buildInfoLabels = []string{common.Job, common.Instance, gitVersion, major, minor}

func getVersion(cluster string, result model.Matrix) {
	var cvs *ClusterVersions
	if cvs = clusterKubernetesVersions[cluster]; cvs == nil {
		cvs = &ClusterVersions{ApiServers: NewTypedVersions(), Nodes: NewTypedVersions()}
		clusterKubernetesVersions[cluster] = cvs
	}
	for _, ss := range result {
		if bil, f := common.GetLabelsValues(ss, buildInfoLabels); f {
			var tvs *TypedVersions
			if strings.Contains(bil[common.Job], apiServers) {
				tvs = cvs.ApiServers
			} else if strings.Contains(bil[common.Job], nodes) || strings.Contains(bil[common.Job], kubelet) {
				tvs = cvs.Nodes
			} else {
				continue
			}
			tvs.Add(bil[common.Instance],
				&BuildInfo{GitVersion: bil[gitVersion], Major: parseVersion(bil[major]), Minor: parseVersion(bil[minor])})
		}
	}
}

var digitsOnly = regexp.MustCompile("[0-9]+")

func parseVersion(ver string) (n uint64) {
	n, _ = strconv.ParseUint(digitsOnly.FindString(ver), 10, 64)
	return
}
