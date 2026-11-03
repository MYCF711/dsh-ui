//go:build !windows

package main

import "time"

// startShot 在非 Windows 平台上不做任何事（本机的截图方案用的是 GDI）。
func startShot(path string, delay time.Duration) {}
