package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// click presses the middle of the control with the given identity, as a user would.
func click(t *testing.T, driver *app.Driver, id ui.ID) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control %v on screen", id)
}

func shows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func TestButtonsChangeTheCount(t *testing.T) {
	m := &model{}
	var canvas ui.Canvas
	driver, err := app.NewDriver(newConfig(m), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	for want := 1; want <= 3; want++ {
		click(t, driver, increment)
		if !shows(driver, fmt.Sprintf("Count: %d", want)) {
			t.Fatalf("after %d clicks the window does not show the count", want)
		}
	}
	click(t, driver, reset)
	if !shows(driver, "Count: 0") {
		t.Fatal("Reset did not zero the count")
	}
}
