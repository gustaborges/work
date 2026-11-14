package marketplace

import "sort"

var catalogKeys = []string{"schema", "name", "display_name", "description", "plugins"}
var listingKeys = []string{"name", "path", "description"}

type Descriptor struct {
	CatalogKeys, ListingKeys []string
	Schema                   int
}

func Contract() Descriptor {
	a := append([]string{}, catalogKeys...)
	b := append([]string{}, listingKeys...)
	sort.Strings(a)
	sort.Strings(b)
	return Descriptor{a, b, 1}
}
