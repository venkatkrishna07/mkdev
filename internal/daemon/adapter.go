package daemon

import (
	"strings"
	"time"

	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

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
		Enabled:  r.Enabled,
	}
}

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

func SplitDomain(domain, tld string) string {
	name, _ := strings.CutSuffix(domain, tld)
	return name
}
