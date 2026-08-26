//go:build darwin && !ios && !server

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <Cocoa/Cocoa.h>
#include <stdlib.h>

static void flyqproSetProcessName(const char *name) {
	if (name == NULL) {
		return;
	}

	NSString *title = [NSString stringWithUTF8String:name];
	if (title != nil) {
		[[NSProcessInfo processInfo] setProcessName:title];
	}
}

static void flyqproSetApplicationMenuName(const char *name) {
	if (name == NULL || NSApp == nil) {
		return;
	}

	NSMenu *menu = [NSApp mainMenu];
	if (menu == nil || [menu numberOfItems] == 0) {
		return;
	}

	NSString *title = [NSString stringWithUTF8String:name];
	if (title != nil) {
		[[menu itemAtIndex:0] setTitle:title];
	}
}
*/
import "C"

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// configureNativeApplicationName covers direct binary runs as well as .app
// bundles. A raw macOS executable has no Info.plist, so AppKit can otherwise
// derive the first application-menu title from the FlyQPro process name.
func configureNativeApplicationName(app *application.App) {
	name := C.CString("飞秋Pro")
	C.flyqproSetProcessName(name)
	C.free(unsafe.Pointer(name))

	app.Event.OnApplicationEvent(events.Mac.ApplicationDidFinishLaunching, func(*application.ApplicationEvent) {
		application.InvokeSync(func() {
			menuName := C.CString("飞秋Pro")
			defer C.free(unsafe.Pointer(menuName))
			C.flyqproSetApplicationMenuName(menuName)
		})
	})
}
