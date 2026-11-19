module github.com/MYCF711/dsh-ui

go 1.27.3

require (
	github.com/charmbracelet/colorprofile v0.4.2 // indirect
	github.com/charmbracelet/x/exp/ordered v0.1.0 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.0 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
)

require (
	github.com/charmbracelet/ultraviolet v0.0.0-20260303162955-0b88c25f3fff
	github.com/charmbracelet/x/ansi v0.11.7
	github.com/charmbracelet/x/vt v0.1.0
	github.com/coder/websocket v1.8.15
	github.com/klauspost/compress v1.20.3
	github.com/vivaui/viva v0.6.5
)

replace example.com/dsh-hello => ../dsh-ui-plugins/hello

// Viva: own fork branch carrying vivaui/viva#355 (DropdownMenuOptions.Shadow /
// RowHeight). PR need not be merged for us to consume it: replace pins the fork
// pseudo-version instead of waiting on upstream review.
// Retire this replace once vivaui/viva#355 merges: drop it, then
// `go get github.com/vivaui/viva@<merged-tag>` and `go mod tidy`.
replace github.com/vivaui/viva => github.com/MYCF711/viva v0.6.6-0.20261119223403-183013a381da
