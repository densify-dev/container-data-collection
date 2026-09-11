package container

import (
	"encoding/json"
	"strconv"
	"sync"

	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
)

// RuntimeDetails will be enriched in the future when we get specific details (e.g. GC settings)
// for each specific Runtime.
// The full list of potential runtimes is at https://github.com/open-telemetry/opentelemetry-go/blob/main/semconv/v1.38.0/attribute_group.go#L14059
// (the semconv version may change with https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/)
type RuntimeDetails interface {
	runtimeDetails()
}

type RuntimeDetailsDTO struct {
	Type string         `json:"type"`
	Data RuntimeDetails `json:"data,omitempty"`
}

type Runtime struct {
	Name           string            `json:"name"`
	Version        string            `json:"version,omitempty"`
	RuntimeDetails RuntimeDetailsDTO `json:"runtimeDetails,omitempty"`
	fpOnce         sync.Once         `json:"-"`
	fingerprint    uint64            `json:"-"`
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
	byName       map[string][]int
}

func initRuntimes() *Runtimes {
	return &Runtimes{
		runtimes:     make([]*Runtime, 0),
		fingerprints: make(map[uint64]bool),
		byName:       make(map[string][]int),
	}
}

func (rs *Runtimes) addRuntime(r *Runtime) {
	rs.add(r, common.UnknownValue)
}

func (rs *Runtimes) updateRuntime(r *Runtime) {
	if !r.IsValid() {
		return
	}
	if currR, idx, f := rs.getRuntimeIdx(r.Name); f {
		delete(rs.fingerprints, currR.Fingerprint())
		rs.add(r, idx)
	} else {
		rs.addRuntime(r)
	}
}

func (rs *Runtimes) getRuntime(name string) (r *Runtime, ok bool) {
	r, _, ok = rs.getRuntimeIdx(name)
	return
}

func (rs *Runtimes) getRuntimeIdx(name string) (r *Runtime, idx int, ok bool) {
	if l := len(rs.byName[name]); l > 0 {
		idx = rs.byName[name][0]
		r, ok = rs.runtimes[idx], true
	} else {
		idx = common.UnknownValue
	}
	return

}

func (rs *Runtimes) add(r *Runtime, idx int) {
	if !r.IsValid() {
		return
	}
	fp := r.Fingerprint()
	if !rs.fingerprints[fp] {
		if idx == common.UnknownValue {
			rs.runtimes = append(rs.runtimes, r)
			rs.byName[r.Name] = append(rs.byName[r.Name], len(rs.runtimes)-1)
		} else {
			rs.runtimes[idx] = r
		}
		rs.fingerprints[fp] = true
	}
}

func (r *Runtime) IsValid() bool {
	return r != nil && r.Name != common.Empty
}

type RuntimeProcessFields struct {
	CommandArgs []string          `json:"commandArgs,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	CommandExe  string            `json:"commandExe,omitempty"`
	Pid         int               `json:"pid,omitempty"`
}

const (
	argsLabel = "process_command_args"
	exeLabel  = "process_executable_path"
	pidLabel  = "process_pid"
)

func parseProcessRuntimeFields(ss *model.SampleStream) (rtp *RuntimeProcessFields, err error) {
	var args []string
	var argsLabelValue, exePath, pidStr string
	var pid int
	var ok bool
	if argsLabelValue, ok = common.GetLabelValue(ss, argsLabel); ok {
		if err = json.Unmarshal([]byte(argsLabelValue), &args); err != nil {
			return
		}
	}
	exePath, _ = common.GetLabelValue(ss, exeLabel)
	if pidStr, ok = common.GetLabelValue(ss, pidLabel); ok {
		if pid, err = strconv.Atoi(pidStr); err != nil {
			return
		}
	}
	rtp = &RuntimeProcessFields{
		CommandArgs: args,
		CommandExe:  exePath,
		Pid:         pid,
	}
	return
}
