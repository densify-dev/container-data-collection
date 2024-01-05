package common

import (
	"fmt"
	"github.com/samber/lo"
	"golang.org/x/exp/constraints"
	"sort"
)

// Number -
// Ideally we would define Number as
//
//	type Number interface {
//	    constraints.Integer | constraints.Float
//	}
//
// However, this doesn't work, as in some cases we have arithmetic expressions involving constant values which
// exceed int8, int16, unit8, uint16 - therefore we need to exclude these types (to avoid compilation errors).
// In addition, constraints.Integer (specifically constraints.Unsigned) includes ~uintptr which we want to
// exclude as well.
type Number interface {
	~int | ~uint | ~int32 | ~uint32 | ~int64 | ~uint64 | ~float32 | ~float64
}

func IsSubset[K, V comparable](m, sub map[K]V) bool {
	if len(sub) > len(m) {
		return false
	}
	for k, v := range sub {
		if !Contains(m, k, v) {
			return false
		}
	}
	return true
}

func Contains[K, V comparable](m map[K]V, k K, v V) bool {
	vm, found := m[k]
	return found && vm == v
}

// KeySet returns the key set of m, nil if m is empty or nil
func KeySet[K comparable, V any](m map[K]V) (keys []K) {
	if len(m) > 0 {
		keys = lo.Keys(m)
	}
	return
}

func SortedKeySet[K constraints.Ordered, V any](m map[K]V) (keys []K) {
	keys = KeySet(m)
	SortSlice(keys)
	return
}

func SortSlice[T constraints.Ordered](s []T) {
	if len(s) > 0 {
		sort.Slice(s, func(i, j int) bool {
			return s[i] < s[j]
		})
	}
}

type MergeDuplicateStrategy int

const (
	Fail MergeDuplicateStrategy = iota
	Ignore
	Override
)

func Merge[K comparable, V any](m1, m2 map[K]V, mds MergeDuplicateStrategy) (m map[K]V, err error) {
	m = make(map[K]V, len(m1)+len(m2))
	for k, v := range m1 {
		m[k] = v
	}
m2Loop:
	for k, v := range m2 {
		if _, found := m[k]; found {
			switch mds {
			case Fail:
				err = fmt.Errorf("duplicate entries for key %s", k)
				m = nil
				break m2Loop
			case Override:
				m[k] = v
			default:
			}
		} else {
			m[k] = v
		}
	}
	return
}
