package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")

func configureDPI() {
	p := user32.NewProc("SetProcessDpiAwarenessContext")
	if p.Find() == nil {
		p.Call(^uintptr(3))
	}
}
func darkTitlebar(hwnd unsafe.Pointer) {
	p := syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	on := uint32(1)
	p.Call(uintptr(hwnd), 20, uintptr(unsafe.Pointer(&on)), 4)
}

func placeWindow(hwnd unsafe.Pointer) {
	type rect struct{ Left, Top, Right, Bottom int32 }
	type monitorInfo struct {
		Size          uint32
		Monitor, Work rect
		Flags         uint32
	}
	var bounds rect
	user32.NewProc("GetWindowRect").Call(uintptr(hwnd), uintptr(unsafe.Pointer(&bounds)))
	monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(uintptr(hwnd), 2)
	info := monitorInfo{}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
		aw, ah := info.Work.Right-info.Work.Left, info.Work.Bottom-info.Work.Top
		if width > aw-32 {
			width = aw - 32
		}
		if height > ah-32 {
			height = ah - 32
		}
		x, y := info.Work.Left+(aw-width)/2, info.Work.Top+(ah-height)/2
		user32.NewProc("SetWindowPos").Call(uintptr(hwnd), 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x14)
	}
	module, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	for _, size := range []uintptr{16, 32} {
		icon, _, _ := user32.NewProc("LoadImageW").Call(module, 1, 1, size, size, 0)
		if icon != 0 {
			kind := uintptr(0)
			if size == 32 {
				kind = 1
			}
			user32.NewProc("SendMessageW").Call(uintptr(hwnd), 0x80, kind, icon)
		}
	}
}

type openFileName struct {
	Size          uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustom     uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	Template      *uint16
	Reserved      unsafe.Pointer
	Reserved2     uint32
	FlagsEx       uint32
}

func fileDialog(hwnd unsafe.Pointer, save bool, name, ext string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(name)))
	filter := "Логи OpenDiag (*.log;*.txt)\x00*.log;*.txt\x00Все файлы\x00*.*\x00\x00"
	title := "Открыть лог OpenDiag"
	if save {
		filter = stringsForExport(ext)
		title = "Экспорт диагностических данных"
	}
	ff := utf16.Encode([]rune(filter))
	tt, _ := syscall.UTF16PtrFromString(title)
	ee, _ := syscall.UTF16PtrFromString(ext)
	ofn := openFileName{Owner: uintptr(hwnd), Filter: &ff[0], FilterIndex: 1, File: &buf[0], MaxFile: uint32(len(buf)), Title: tt, DefExt: ee, Flags: 0x80000 | 0x800 | 0x8 | 0x10000000}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	proc := "GetOpenFileNameW"
	if save {
		proc = "GetSaveFileNameW"
		ofn.Flags |= 2
	} else {
		ofn.Flags |= 0x1000
	}
	dll := syscall.NewLazyDLL("comdlg32.dll")
	ret, _, _ := dll.NewProc(proc).Call(uintptr(unsafe.Pointer(&ofn)))
	runtime.KeepAlive(ff)
	runtime.KeepAlive(buf)
	if ret == 0 {
		code, _, _ := dll.NewProc("CommDlgExtendedError").Call()
		if code != 0 {
			return "", fmt.Errorf("ошибка диалога Windows: 0x%X", code)
		}
		return "", nil
	}
	return syscall.UTF16ToString(buf), nil
}
func stringsForExport(ext string) string {
	if ext == "txt" {
		return "Текстовый отчёт UTF-8 (*.txt)\x00*.txt\x00\x00"
	}
	return "Таблица CSV UTF-8 (*.csv)\x00*.csv\x00\x00"
}
