package dsfiles

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-files", func() pluginapi.Plugin { return &Plugin{} }) }
