package bar

import (
	"sort"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

func sortRoutes(rs []api.Route) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Name < rs[j].Name })
}
