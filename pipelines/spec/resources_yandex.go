package spec

import (
	"fmt"
	"math"
	"slices"
)

// Snapshot of documented CPU/RAM combinations, checked 2026-09-16.
// https://yandex.cloud/en/docs/compute/concepts/performance-levels
// Unknown platforms are reported as unverified rather than silently constrained
// by a different platform's limits. The provider remains authoritative.
func checkYandexMachine(m Machine, p string, add func(string, string, string, string)) {
	fraction := m.CoreFraction
	if fraction == 0 {
		fraction = 100
	}
	var cores []int
	maxRatio, maxRAM := 0.0, 0
	known := true
	switch m.InstanceType {
	case "standard-v1":
		cores = []int{2, 4, 6, 8, 10, 12, 14, 16, 20, 24, 28, 32}
		maxRatio, maxRAM = 8, 256
	case "standard-v2":
		cores = standardCores(80)
		maxRatio, maxRAM = 16, 1280
	case "standard-v3":
		cores = standardCores(96)
		maxRatio, maxRAM = 16, 640
		limits := map[int]float64{24: 13, 28: 11, 32: 10, 44: 14, 48: 13, 52: 12, 56: 11, 60: 10, 64: 10, 68: 9, 72: 8, 76: 8, 80: 8, 84: 7, 88: 7, 92: 6, 96: 6}
		if v, ok := limits[m.CPU]; ok {
			maxRatio = v
		}
	case "highfreq-v3":
		cores = standardCores(56)
		maxRatio, maxRAM = 16, 448
	case "amd-v1":
		cores = []int{2, 4, 8, 16, 32, 48, 64, 96, 128}
		maxRatio, maxRAM = 6, 768
	case "standard-v4a":
		cores = []int{2, 4, 8, 16, 32, 48, 64, 96, 128, 224, 256, 288}
		maxRatio, maxRAM = 8, 1792
	case "highfreq-v4a":
		cores = []int{2, 4, 8, 16, 24, 32, 48, 80}
		maxRatio, maxRAM = 12, 960
	default:
		known = false
	}
	if !known {
		add(p+".instance_type", "platform_unverified", "WARNING", "CPU/RAM combinations for this platform require provider validation")
		return
	}
	ratio := float64(m.MemoryGB) / float64(m.CPU)
	valid := slices.Contains(cores, m.CPU) && m.MemoryGB <= maxRAM
	if fraction < 100 {
		levels := []int{20, 50}
		if m.InstanceType == "standard-v1" {
			levels = []int{5, 20}
		}
		if m.InstanceType == "standard-v2" {
			levels = []int{5, 20, 50}
		}
		if m.InstanceType == "highfreq-v3" {
			levels = nil
		}
		valid = valid && slices.Contains(levels, fraction) && (m.CPU == 2 || m.CPU == 4)
		low, high := 0.5, 4.0
		if fraction == 5 {
			high = 2
			if m.InstanceType == "standard-v2" {
				low = 0.25
			}
		}
		if fraction == 20 && (m.InstanceType == "standard-v4a" || m.InstanceType == "highfreq-v4a") {
			high = 2
		}
		if fraction == 20 && m.InstanceType == "amd-v1" {
			high = 1.5
		}
		valid = valid && ratio >= low && ratio <= high && (ratio*2 == math.Trunc(ratio*2) || ratio == 0.25)
	} else {
		valid = valid && fraction == 100 && ratio >= 1 && ratio <= maxRatio && ratio == math.Trunc(ratio)
		if m.InstanceType == "standard-v4a" {
			ratios := []float64{1, 2, 4, 8}
			if m.CPU >= 256 {
				ratios = []float64{1, 2, 4}
			}
			valid = valid && slices.Contains(ratios, ratio)
		}
		if m.InstanceType == "highfreq-v4a" {
			valid = valid && slices.Contains([]float64{1, 2, 4, 8, 10, 12}, ratio)
		}
	}
	if !valid {
		add(p, "yc_machine_shape", "ERROR", fmt.Sprintf("%s does not support %d vCPU / %d GiB / %d%% guaranteed CPU", m.InstanceType, m.CPU, m.MemoryGB, fraction))
	}
}

func standardCores(maxCPU int) []int {
	out := []int{2, 4, 6, 8, 10, 12, 14, 16}
	for n := 20; n <= maxCPU; n += 4 {
		out = append(out, n)
	}
	return out
}

// DiskBlockSize preserves explicit settings; otherwise large network disks use
// the smallest documented physical block size that can address their capacity.
func DiskBlockSize(gb, explicit int) int {
	if explicit != 0 {
		return explicit
	}
	block := 4096
	for gb > block*2 && block < 131072 {
		block *= 2
	}
	return block
}

// doc: https://yandex.cloud/en/docs/compute/concepts/disk
func checkDisk(provider ProviderKind, d Disk, p string, add func(string, string, string, string)) {
	if provider == ProviderAWS {
		if d.BlockSize != 0 {
			add(p+".block_size", "provider_option", "ERROR", "AWS EBS does not expose physical block size")
		}
		return
	}
	if provider != ProviderYandex {
		return
	}
	block := DiskBlockSize(d.GB, d.BlockSize)
	if !slices.Contains([]int{4096, 8192, 16384, 32768, 65536, 131072}, block) {
		add(p+".block_size", "block_size", "ERROR", "block size must be a power of two between 4096 and 131072")
	}
	switch d.Type {
	case "network-ssd-nonreplicated", "network-ssd-io-m3":
		if d.GB%93 != 0 {
			add(p+".gb", "disk_granularity", "ERROR", "non-replicated and ultra high-speed SSD size must be a multiple of 93 GiB")
		}
	case "network-ssd", "network-hdd":
		if d.GB > block*2 {
			add(p+".block_size", "disk_addressability", "ERROR", "disk size exceeds the addressable capacity of the selected block size")
		}
	default:
		add(p+".type", "disk_type_unverified", "WARNING", "disk type is not in the local YC capability snapshot; provider validation is required")
	}
}
