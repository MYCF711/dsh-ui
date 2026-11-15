package dsfileactions

import "github.com/MYCF711/dsh-ui/internal/pluginapi"

func init() {
	pluginapi.Register(ServiceID, func() pluginapi.Plugin { return &Plugin{} })
}
