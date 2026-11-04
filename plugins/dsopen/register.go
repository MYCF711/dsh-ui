package dsopen

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-open", func() pluginapi.Plugin { return &Plugin{} }) }
