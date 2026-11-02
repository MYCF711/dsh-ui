module github.com/MYCF711/dsh-ui/spike/e2

go 1.27.3

tool github.com/vivaui/viva/cmd/viva

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
)

require (
	github.com/MYCF711/dsh-ui v0.0.0
	github.com/vivaui/viva v0.2.3
)

replace github.com/MYCF711/dsh-ui => ../..
