package dsbrand

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-brand", func() pluginapi.Plugin { return &Plugin{} }) }
