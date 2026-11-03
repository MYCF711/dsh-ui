package dscommands

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-commands", func() pluginapi.Plugin { return &Plugin{} }) }
