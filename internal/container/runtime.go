package container

import (
	"encoding/json"
	"sync"

	"github.com/densify-dev/container-data-collection/internal/common"
)

// RuntimeDetails will be enriched in the future when we get specific details (e.g. GC settings)
// for each specific Runtime.
// The full list of potential runtimes is at https://github.com/open-telemetry/opentelemetry-go/blob/main/semconv/v1.38.0/attribute_group.go#L14059
// (the semconv version may change with https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/)
type RuntimeDetails interface {
	runtimeDetails()
}

type Runtime struct {
	Name           string         `json:"name"`
	Version        string         `json:"version,omitempty"`
	RuntimeDetails RuntimeDetails `json:"runtimeDetails,omitempty"`
	fpOnce         sync.Once      `json:"-"`
	fingerprint    uint64         `json:"-"`
}

func (r *Runtime) Fingerprint() uint64 {
	r.fpOnce.Do(r.initFingerprint)
	return r.fingerprint
}

func (r *Runtime) initFingerprint() {
	if jsonData, err := json.Marshal(r); err == nil {
		r.fingerprint = common.Fingerprint([]string{string(jsonData)})
	}
}

type Runtimes struct {
	runtimes     []*Runtime
	fingerprints map[uint64]bool
}

func (rs *Runtimes) addRuntime(r *Runtime) {
	fp := r.Fingerprint()
	if !rs.fingerprints[fp] {
		rs.runtimes = append(rs.runtimes, r)
		rs.fingerprints[fp] = true
	}
}
