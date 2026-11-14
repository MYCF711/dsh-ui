package dscontextmenu

import "github.com/MYCF711/dsh-ui/internal/pluginapi"

func init() {
	pluginapi.Register("ds-contextmenu", func() pluginapi.Plugin { return &Plugin{} })
}
