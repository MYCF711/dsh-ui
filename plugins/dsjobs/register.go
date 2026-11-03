package dsjobs

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-jobs", func() pluginapi.Plugin { return &Plugin{} }) }
