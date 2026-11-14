package pluginid

import "sort"

type Descriptor struct {
	Grammar  string
	MaxBytes int
	Reserved string
	Devices  []string
}

func Contract() Descriptor {
	devices := []string{}
	for k := range windowsDevice {
		devices = append(devices, k)
	}
	sort.Strings(devices)
	return Descriptor{nameGrammar.String(), maxNameBytes, Builtin, devices}
}
