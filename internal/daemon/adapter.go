package daemon

import (
	"strings"
	"time"

	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

// APIFromStore converts a store record into a wire DTO.
// Health is always HealthUnknown in M1; later milestones set it from the prober.
func APIFromStore(r store.Route) api.Route {
	share := api.ShareNone
	if r.Shared {
		share = api.ShareLAN
	}
	return api.Route{
		Name:     SplitDomain(r.Domain, r.TLD),
		Target:   r.Target,
		Share:    share,
		Health:   api.HealthUnknown,
		Insecure: r.Insecure,
	}
}

// StoreFromAPI converts a wire DTO into a store record using the supplied TLD.
func StoreFromAPI(r api.Route, tld string) store.Route {
	return store.Route{
		Domain:   r.Name + tld,
		Target:   r.Target,
		TLD:      tld,
		Enabled:  true,
		Shared:   r.Share == api.ShareLAN,
		Insecure: r.Insecure,
		Source:   "ad-hoc",
		AddedAt:  time.Now(),
	}
}

// SplitDomain returns the name portion of a domain by trimming the TLD suffix.
// If domain does not end in tld, the original string is returned unchanged.
func SplitDomain(domain, tld string) string {
	name, _ := strings.CutSuffix(domain, tld)
	return name
}
