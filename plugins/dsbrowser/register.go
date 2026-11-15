package dsbrowser

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-browser", func() pluginapi.Plugin { return &Plugin{} }) }
