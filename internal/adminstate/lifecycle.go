package adminstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"

	"github.com/gustaborges/work/internal/jsonx"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

// ReferencePlugin is the name of the included reference package.
const ReferencePlugin = "work-reference"

// ReferenceProvider is the included reference package's identity.
var ReferenceProvider = pluginid.Provider{Marketplace: pluginid.Builtin, Plugin: ReferencePlugin}

// LifecycleSchema is the only supported lifecycle schema.
const LifecycleSchema = 1

// Lifecycle is the durable default-package history, kept outside any removable
// package directory. Fresh homes have every flag false.
//
// PolicyHandled records that the repository-resolution policy has been
// deliberately decided: either first setup seeded it or the user made an
// explicit choice (including choosing an empty policy). Registering a source
// never changes it. Removed records a deliberate removal of the default
// package so it is never silently resurrected.
type Lifecycle struct {
	Schema        int  `json:"schema"`
	Initialized   bool `json:"initialized"`
	PolicyHandled bool `json:"policy_handled"`
	Removed       bool `json:"removed"`
}

// loadLifecycle reads the lifecycle file. A missing file is the fresh default;
// a present file must match the current strict shape exactly.
func loadLifecycle(h workhome.Home) (*Lifecycle, error) {
	b, err := os.ReadFile(h.LifecycleFile())
	if errors.Is(err, os.ErrNotExist) {
		return &Lifecycle{Schema: LifecycleSchema}, nil
	}
	if err != nil {
		return nil, recoveryError(err)
	}
	return decodeLifecycle(b)
}

func decodeLifecycle(b []byte) (*Lifecycle, error) {
	var l Lifecycle
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil || jsonx.TrailingData(dec) || l.Schema != LifecycleSchema || (l.Removed && !l.Initialized) {
		return nil, recoveryError(errors.New("invalid lifecycle state"))
	}
	return &l, nil
}

// MarkRemoved records a deliberate removal of the default package.
func (l *Lifecycle) MarkRemoved() {
	l.Initialized = true
	l.Removed = true
}

// MarkPolicyHandled records an explicit repository-policy choice. The caller
// persists the lifecycle image in the same transaction as the choice, so an
// explicit empty policy is meaningful even when the Locator list was already
// empty and first-use setup can never overwrite it with the default.
func (l *Lifecycle) MarkPolicyHandled() { l.PolicyHandled = true }
