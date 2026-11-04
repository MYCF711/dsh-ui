package dsterminal

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-terminal", func() pluginapi.Plugin { return &Plugin{} }) }
