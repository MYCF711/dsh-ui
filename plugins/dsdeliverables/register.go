package dsdeliverables

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-deliverables", func() pluginapi.Plugin { return &Plugin{} }) }
