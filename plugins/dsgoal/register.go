package dsgoal

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-goal", func() pluginapi.Plugin { return &Plugin{} }) }
