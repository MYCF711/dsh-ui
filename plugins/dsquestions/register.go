package dsquestions

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-questions", func() pluginapi.Plugin { return &Plugin{} }) }
