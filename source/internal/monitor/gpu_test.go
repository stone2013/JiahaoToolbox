package monitor

import (
	"math"
	"testing"
)

func TestGPUCounterKey(t *testing.T) {
	id, node, engine, ok := gpuCounterKey("pid_100_luid_0x00000001_0x00AbCdEf_phys_0_eng_2_engtype_3D")
	if !ok || id != "0000000100abcdef" || node != "0" || engine != "2" {
		t.Fatal(id, node, engine, ok)
	}
	if _, _, _, ok = gpuCounterKey("unknown"); ok {
		t.Fatal("invalid name accepted")
	}
}
func TestGPUBusiestEngineNotSum(t *testing.T) {
	m := map[string]float64{"pid_1_luid_0x0_0x1234_phys_0_eng_0_engtype_3D": 30, "pid_2_luid_0x0_0x1234_phys_0_eng_0_engtype_3D": 40, "pid_3_luid_0x0_0x1234_phys_0_eng_1_engtype_Copy": 50, "pid_4_luid_0x0_0x5678_phys_0_eng_0_engtype_3D": 90, "bad": 100}
	v := gpuLoads(m)
	assertValue(t, v["0000000000001234"], 70)
	assertValue(t, v["0000000000005678"], 90)
	if len(v) != 2 {
		t.Fatal(v)
	}
}
func TestGPUMissingAndInvalid(t *testing.T) {
	if len(gpuLoads(nil)) != 0 {
		t.Fatal("missing counters must not become zero")
	}
	m := map[string]float64{"pid_1_luid_0x0_0x1_phys_0_eng_0": math.NaN(), "pid_2_luid_0x0_0x1_phys_0_eng_0": -1}
	if len(gpuLoads(m)) != 0 {
		t.Fatal("invalid counters")
	}
}
func TestGPUMemory64Bit(t *testing.T) {
	m := map[string]float64{"luid_0x0_0x1_phys_0": 8 * 1024 * 1024 * 1024, "luid_0x0_0x1_phys_1": 4 * 1024 * 1024 * 1024}
	assertValue(t, gpuMemory(m)["0000000000000001"], 12*1024*1024*1024)
}
