package main

import "github.com/vivaui/viva/ui"

var (
	tree = ui.Tree{}
	home = tree.Feature("home")

	btnNew    = tree.ID(home, "new")
	sideScr   = tree.ID(home, "side")
	msgScr    = tree.ID(home, "msgs")
	draftID   = tree.ID(home, "draft")
	btnSend   = tree.ID(home, "send")
	searchID  = tree.ID(home, "search")
	btnAllow  = tree.ID(home, "allow")
	btnDeny   = tree.ID(home, "deny")
	btnAlways = tree.ID(home, "always")
)
