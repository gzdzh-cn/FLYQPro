//go:build windows

package main

import (
	"log"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

const windowsAppUserModelID = "DZH.FlyQPro"

var (
	clsidDestinationList            = ole.NewGUID("{77F10CF0-3DB5-4966-B520-B7C54E9A0B35}")
	iidCustomDestination            = ole.NewGUID("{6332DEBF-87B5-4670-90C0-5E57B408A49E}")
	iidObjectArray                  = ole.NewGUID("{92CA9DCD-5622-4BBA-A805-5E9F541BD8C9}")
	clsidEnumerableObjectCollection = ole.NewGUID("{2D3468C1-36A7-43B6-AC24-D3F02FD9607A}")
	iidObjectCollection             = ole.NewGUID("{5632B1A4-E38A-400A-928A-D4CD63230295}")
	clsidShellLink                  = ole.NewGUID("{00021401-0000-0000-C000-000000000046}")
	iidShellLinkW                   = ole.NewGUID("{000214F9-0000-0000-C000-000000000046}")
	iidPropertyStore                = ole.NewGUID("{886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99}")
	taskbarCreatedMessage           = w32.RegisterWindowMessage(w32.MustStringToUTF16Ptr("TaskbarCreated"))
	taskbarReloadPending            atomic.Bool
)

const (
	jumpListOpenArg = "--flyqpro-task=open"
	jumpListQuitArg = "--flyqpro-task=quit"
	vtLPWSTR        = 31
)

type propertyKey struct {
	fmtid ole.GUID
	pid   uint32
}

// propVariant is the Windows PROPVARIANT layout for a VT_LPWSTR value.
type propVariant struct {
	vt         uint16
	wReserved1 uint16
	wReserved2 uint16
	wReserved3 uint16
	value      uintptr
	padding    uintptr
}

func configureWindowsAppIdentity() {
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	setAppID := shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	appID, err := windows.UTF16PtrFromString(windowsAppUserModelID)
	if err != nil {
		log.Printf("创建 Windows AppUserModelID 失败: %v", err)
		return
	}
	result, _, callErr := setAppID.Call(uintptr(unsafe.Pointer(appID)))
	if result != 0 {
		if callErr != syscall.Errno(0) {
			log.Printf("设置 Windows AppUserModelID 失败: %v", callErr)
		} else {
			log.Printf("设置 Windows AppUserModelID 失败: HRESULT 0x%X", result)
		}
	}
}

func configureWindowsTaskbar(_ *application.App, _ *application.WebviewWindow) func() {
	if err := installWindowsJumpList(); err != nil {
		log.Printf("设置 Windows 任务栏菜单失败: %v", err)
	}
	return func() {}
}

func configureWindowsSystemTray(app *application.App, mainWindow *application.WebviewWindow) func() {
	tray := app.SystemTray.New()
	showMainWindow := func() {
		mainWindow.UnMinimise()
		mainWindow.Show()
		mainWindow.Focus()
	}

	menu := app.NewMenu()
	menu.Add("打开飞秋Pro").OnClick(func(_ *application.Context) {
		showMainWindow()
	})
	menu.AddSeparator()
	menu.Add("退出飞秋Pro").OnClick(func(_ *application.Context) {
		app.Quit()
	})

	tray.SetIcon(appIcon).SetMenu(menu).OnClick(showMainWindow).OnDoubleClick(showMainWindow).OnRightClick(tray.ShowMenu)
	tray.SetTooltip("飞秋Pro")

	return func() {
		tray.Destroy()
	}
}

func windowsTaskbarWndProcInterceptor() func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (uintptr, bool) {
	return func(_ uintptr, msg uint32, _, _ uintptr) (uintptr, bool) {
		if msg != taskbarCreatedMessage || taskbarReloadPending.Swap(true) {
			return 0, false
		}
		go func() {
			time.Sleep(time.Second)
			if err := installWindowsJumpList(); err != nil {
				log.Printf("任务栏重启后重新设置菜单失败: %v", err)
			}
			taskbarReloadPending.Store(false)
		}()
		return 0, false
	}
}

func installWindowsJumpList() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	initialized := false
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 1 {
			return err
		}
	}
	initialized = true
	if initialized {
		defer ole.CoUninitialize()
	}

	destination, err := ole.CreateInstance(clsidDestinationList, iidCustomDestination)
	if err != nil {
		return err
	}
	defer destination.Release()

	if _, err := comCall(destination, 3, utf16Pointer(windowsAppUserModelID)); err != nil {
		return err
	}

	var maxSlots uint32
	var removed *ole.IUnknown
	if _, err := comCall(destination, 4,
		uintptr(unsafe.Pointer(&maxSlots)),
		uintptr(unsafe.Pointer(iidObjectArray)),
		uintptr(unsafe.Pointer(&removed)),
	); err != nil {
		return err
	}
	if removed != nil {
		removed.Release()
	}

	tasks, err := ole.CreateInstance(clsidEnumerableObjectCollection, iidObjectCollection)
	if err != nil {
		return err
	}
	defer tasks.Release()

	for _, task := range []struct {
		title string
		arg   string
	}{
		{title: "打开飞秋Pro", arg: jumpListOpenArg},
		{title: "退出飞秋Pro", arg: jumpListQuitArg},
	} {
		link, linkErr := newJumpListShellLink(executable, task.title, task.arg)
		if linkErr != nil {
			return linkErr
		}
		addErr := comCallVoid(tasks, 5, uintptr(unsafe.Pointer(link)))
		link.Release()
		if addErr != nil {
			return addErr
		}
	}

	if _, err := comCall(destination, 7, uintptr(unsafe.Pointer(tasks))); err != nil {
		return err
	}
	_, err = comCall(destination, 8)
	return err
}

func newJumpListShellLink(executable, title, arguments string) (*ole.IUnknown, error) {
	link, err := ole.CreateInstance(clsidShellLink, iidShellLinkW)
	if err != nil {
		return nil, err
	}
	fail := func(callErr error) (*ole.IUnknown, error) {
		link.Release()
		return nil, callErr
	}

	if _, err := comCall(link, 20, utf16Pointer(executable)); err != nil {
		return fail(err)
	}
	if _, err := comCall(link, 11, utf16Pointer(arguments)); err != nil {
		return fail(err)
	}
	if _, err := comCall(link, 7, utf16Pointer(title)); err != nil {
		return fail(err)
	}

	propertyStore, err := queryInterface(link, iidPropertyStore)
	if err != nil {
		return fail(err)
	}
	defer propertyStore.Release()

	titleKey := propertyKey{
		fmtid: *ole.NewGUID("{F29F85E0-4FF9-1068-AB91-08002B27B3D9}"),
		pid:   2,
	}
	titleValue := utf16Pointer(title)
	value := propVariant{vt: vtLPWSTR, value: uintptr(unsafe.Pointer(titleValue))}
	if _, err := comCall(propertyStore, 6,
		uintptr(unsafe.Pointer(&titleKey)),
		uintptr(unsafe.Pointer(&value)),
	); err != nil {
		return fail(err)
	}
	if _, err := comCall(propertyStore, 7); err != nil {
		return fail(err)
	}
	return link, nil
}

func queryInterface(source *ole.IUnknown, iid *ole.GUID) (*ole.IUnknown, error) {
	var result *ole.IUnknown
	if err := source.PutQueryInterface(iid, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func comCallVoid(object *ole.IUnknown, index int, args ...uintptr) error {
	_, err := comCall(object, index, args...)
	return err
}

func comCall(object *ole.IUnknown, index int, args ...uintptr) (uintptr, error) {
	if object == nil {
		return 0, ole.NewError(0x80004003)
	}
	vtable := *(*uintptr)(unsafe.Pointer(object))
	method := *(*uintptr)(unsafe.Pointer(vtable + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	callArgs := make([]uintptr, len(args)+1)
	callArgs[0] = uintptr(unsafe.Pointer(object))
	copy(callArgs[1:], args)
	result, _, _ := syscall.SyscallN(method, callArgs...)
	if int32(result) < 0 {
		return result, ole.NewError(result)
	}
	return result, nil
}

func utf16Pointer(value string) uintptr {
	pointer, _ := windows.UTF16PtrFromString(value)
	return uintptr(unsafe.Pointer(pointer))
}
