package dsforge

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-forge", func() pluginapi.Plugin { return &Plugin{} }) }
