module github.com/MYCF711/dsh-ui

go 1.27.3

require github.com/ebitengine/purego v0.11.1 // indirect

require (
	github.com/coder/websocket v1.8.15
	github.com/klauspost/compress v1.20.3
	github.com/vivaui/viva v0.6.0
)

replace example.com/dsh-hello => ../dsh-ui-plugins/hello
