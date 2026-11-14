package dsattachment

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-attachment", func() pluginapi.Plugin { return &Plugin{} }) }
