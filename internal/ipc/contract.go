package ipc

import (
	"reflect"
	"sort"
)

type Field struct{ Name, Type string }
type Descriptor struct {
	Types                   map[string][]Field
	StdoutLimit, StderrTail int
}

// Contract reflects actual JSON types, including nested response fields.
func Contract() Descriptor {
	d := Descriptor{map[string][]Field{}, StdoutLimitBytes, stderrTailBytes}
	var visit func(reflect.Type)
	visit = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		if _, ok := d.Types[t.Name()]; ok {
			return
		}
		d.Types[t.Name()] = []Field{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			d.Types[t.Name()] = append(d.Types[t.Name()], Field{f.Tag.Get("json"), f.Type.String()})
			visit(f.Type)
		}
		sort.Slice(d.Types[t.Name()], func(i, j int) bool { return d.Types[t.Name()][i].Name < d.Types[t.Name()][j].Name })
	}
	for _, v := range []any{StarterInput{}, StarterResponse{}, LocatorInput{}, LocatorResponse{}, LinkerInput{}, LinkerResponse{}, ImporterInput{}} {
		visit(reflect.TypeOf(v))
	}
	return d
}
