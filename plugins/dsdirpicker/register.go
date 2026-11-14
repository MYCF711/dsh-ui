package dsdirpicker

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-dirpicker", func() pluginapi.Plugin { return &Plugin{} }) }
