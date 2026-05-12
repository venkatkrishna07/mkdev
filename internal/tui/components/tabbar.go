package components

import (
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// TabBar renders a single-line tab strip. The active tab is bold + primary
// foreground; inactives are dim. Two spaces separate adjacent labels.
func TabBar(th styles.Theme, labels []string, active int) string {
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == active {
			parts[i] = th.TabActive.Render(l)
		} else {
			parts[i] = th.TabInactive.Render(l)
		}
	}
	return strings.Join(parts, "  ")
}
