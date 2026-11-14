package semconv

import "sort"

type Descriptor struct {
	Facts                                      []string
	PublicGrammar, LocalGrammar, PrivatePrefix string
}

func Contract() Descriptor {
	facts := append([]string{}, Facts...)
	sort.Strings(facts)
	return Descriptor{facts, publicKey.String(), localKey.String(), privatePrefix}
}
