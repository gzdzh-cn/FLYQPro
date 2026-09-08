//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func configureWindowsAppIdentity() {}

func configureWindowsTaskbar(_ *application.App, _ *application.WebviewWindow) func() {
	return func() {}
}

func windowsTaskbarWndProcInterceptor() func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (uintptr, bool) {
	return nil
}
