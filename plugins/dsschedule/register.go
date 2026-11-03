package dsschedule

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-schedule", func() pluginapi.Plugin { return &Plugin{} }) }
