package monitor

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

// The GPU counters are WDDM counters. Their instance names include a LUID;
// pairing by that key avoids guessing based on a GPU's model or enumeration order.
var gpuInstance = regexp.MustCompile(`(?i)luid_0x([0-9a-f]{1,8})_0x([0-9a-f]{1,8})_phys_([0-9]+)(?:_eng_([0-9]+))?`)

func gpuCounterKey(name string) (id, node, engine string, ok bool) {
	m := gpuInstance.FindStringSubmatch(name)
	if len(m) != 5 {
		return
	}
	hi, e := strconv.ParseUint(m[1], 16, 32)
	if e != nil {
		return
	}
	lo, e := strconv.ParseUint(m[2], 16, 32)
	if e != nil {
		return
	}
	return fmt.Sprintf("%08x%08x", hi, lo), m[3], m[4], true
}

// Sum processes using the SAME engine, then choose the busiest engine. Never add
// independent 3D/video/copy engines to fabricate an overall value above 100%.
func gpuLoads(counters map[string]float64) map[string]Value {
	totals := map[string]float64{}
	adapterFor := map[string]string{}
	for name, n := range counters {
		if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			continue
		}
		id, node, engine, ok := gpuCounterKey(name)
		if !ok || engine == "" {
			continue
		}
		key := id + ":" + node + ":" + engine
		totals[key] += n
		adapterFor[key] = id
	}
	out := map[string]Value{}
	for key, n := range totals {
		id := adapterFor[key]
		if n > 100 {
			n = 100
		}
		old, exists := out[id]
		if !exists || n > *old.Value {
			out[id] = Valid(n, "%")
		}
	}
	return out
}
func gpuMemory(counters map[string]float64) map[string]Value {
	totals := map[string]float64{}
	for name, n := range counters {
		id, _, engine, ok := gpuCounterKey(name)
		if !ok || engine != "" || n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			continue
		}
		totals[id] += n
	}
	out := map[string]Value{}
	for id, n := range totals {
		out[id] = Valid(n, "B")
	}
	return out
}
