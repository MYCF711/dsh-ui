package dsvoice

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-voice", func() pluginapi.Plugin { return &Plugin{} }) }
