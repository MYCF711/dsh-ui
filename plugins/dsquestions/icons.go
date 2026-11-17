package dsquestions

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// 通用问题卡的图标全部编译自官方 JSX/sprite 路径，不手画。
// close/chevron-left/chevron-right：assets/official/icons/{settings,composer}/*.jsx；
// chevron-up：rc.2 index-ghOpC9uV.js:129 的 chevron-up-outline；
// check/edit：dsassets 已登记的同一官方原件。
const (
	questionChevronUpPath    = `M12 10L8.70711 6.70711C8.31658 6.31658 7.68342 6.31658 7.29289 6.70711L4 10`
	questionChevronLeftPath  = `M10 4L6.70711 7.29289C6.31658 7.68342 6.31658 8.31658 6.70711 8.70711L10 12`
	questionChevronRightPath = `M6 12L9.29289 8.70711C9.68342 8.31658 9.68342 7.68342 9.29289 7.29289L6 4`
	questionCloseAPath       = `M2.5 2.5L13.5 13.5`
	questionCloseBPath       = `M13.5 2.5L2.5 13.5`
)

func questionStrokeIcon(paths ...string) (gfx.Icon, bool) {
	icon := gfx.Icon{ViewBox: 16, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound}
	for _, path := range paths {
		commands, ok := dsassets.ParsePath(path)
		if !ok {
			return gfx.Icon{}, false
		}
		icon.Commands = append(icon.Commands, commands...)
	}
	return icon, len(icon.Commands) != 0
}

func questionChevronDownIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands}, true
}

func questionChevronUpIcon() (gfx.Icon, bool)    { return questionStrokeIcon(questionChevronUpPath) }
func questionChevronLeftIcon() (gfx.Icon, bool)  { return questionStrokeIcon(questionChevronLeftPath) }
func questionChevronRightIcon() (gfx.Icon, bool) { return questionStrokeIcon(questionChevronRightPath) }
func questionCloseIcon() (gfx.Icon, bool) {
	return questionStrokeIcon(questionCloseAPath, questionCloseBPath)
}
func questionCheckIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.CheckOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands}, true
}
