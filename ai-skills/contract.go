package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/semconv"
)

// ContractBytes is the versioned canonical represented authoring contract.
func ContractBytes() []byte {
	b, err := json.Marshal(struct {
		Version  int
		Plugin   plugin.Descriptor
		Catalog  marketplace.Descriptor
		Identity pluginid.Descriptor
		Semconv  semconv.Descriptor
		IPC      ipc.Descriptor
	}{1, plugin.Contract(), marketplace.Contract(), pluginid.Contract(), semconv.Contract(), ipc.Contract()})
	if err != nil {
		panic(err)
	}
	return b
}
func ContractPin() string {
	h := sha256.Sum256(ContractBytes())
	return "sha256:" + hex.EncodeToString(h[:]) + "\n"
}
func Generated() map[string][]byte {
	out := map[string][]byte{}
	for name, v := range map[string]any{"catalog-schema": marketplace.Contract(), "manifest-schema": plugin.Contract(), "role-matrix": struct {
		Identity pluginid.Descriptor
		Semconv  semconv.Descriptor
		IPC      ipc.Descriptor
	}{pluginid.Contract(), semconv.Contract(), ipc.Contract()}} {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			panic(err)
		}
		out[name+".md"] = fmt.Appendf(nil, "# %s\n\nGenerated from validator/protocol descriptors. Do not edit.\n\n<!-- fixture:descriptor -->\n```json\n%s\n```\n", name, b)
	}
	return out
}
