//go:build windows

package shared

import (
	"fmt"
	"syscall"
	"unsafe"

	"path/filepath"

	"golang.org/x/sys/windows"
)

var (
	ole32COM         = windows.NewLazyDLL("ole32.dll")
	coInitializeEx   = ole32COM.NewProc("CoInitializeEx")
	coUninitialize   = ole32COM.NewProc("CoUninitialize")
	coCreateInstance = ole32COM.NewProc("CoCreateInstance")
	coTaskMemFree    = ole32COM.NewProc("CoTaskMemFree")
)

// GUIDs
var (
	CLSID_FileOpenDialog = windows.GUID{
		Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE,
		Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7},
	}
	IID_IFileOpenDialog = windows.GUID{
		Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768,
		Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60},
	}
	IID_IShellItem = windows.GUID{
		Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE,
		Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE},
	}
)

const (
	CLSCTX_ALL                = 0x17
	FOS_PICKFOLDERS           = 0x00000020 // Makes it a folder picker!
	SIGDN_FILESYSPATH uintptr = 0x80058000
)

// IFileOpenDialog vtable layout (offset order matters!)
type iFileOpenDialogVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	Show                uintptr // index 3
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr // index 9
	GetOptions          uintptr // index 10
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr // index 20
	AddPlace            uintptr
	SetDefaultExtension uintptr
	Close               uintptr
	SetClientGuid       uintptr
	ClearClientData     uintptr
	SetFilter           uintptr
	GetResults          uintptr
	GetSelectedItems    uintptr
}

type iFileOpenDialog struct {
	vtbl *iFileOpenDialogVtbl
}

// IShellItem vtable
type iShellItemVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr // index 5
	GetAttributes  uintptr
	Compare        uintptr
}

type iShellItem struct {
	vtbl *iShellItemVtbl
}

func PickFolder() (string, error) {
	// Init COM
	coInitializeEx.Call(0, 0x2) // COINIT_APARTMENTTHREADED
	defer coUninitialize.Call()

	var dialog *iFileOpenDialog
	ret, _, _ := coCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_FileOpenDialog)),
		0,
		CLSCTX_ALL,
		uintptr(unsafe.Pointer(&IID_IFileOpenDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)
	if ret != 0 {
		return "", fmt.Errorf("CoCreateInstance failed: 0x%X", ret)
	}
	defer syscall.SyscallN(dialog.vtbl.Release, uintptr(unsafe.Pointer(dialog)))

	// ---- To make it a FOLDER picker, uncomment below ----
	var opts uint32
	syscall.SyscallN(dialog.vtbl.GetOptions,
		uintptr(unsafe.Pointer(dialog)),
		uintptr(unsafe.Pointer(&opts)))
	syscall.SyscallN(dialog.vtbl.SetOptions,
		uintptr(unsafe.Pointer(dialog)),
		uintptr(opts|FOS_PICKFOLDERS))

	// Show dialog
	ret, _, _ = syscall.SyscallN(dialog.vtbl.Show,
		uintptr(unsafe.Pointer(dialog)), 0)
	if ret != 0 {
		return "", fmt.Errorf("cancelled")
	}

	// Get result
	var item *iShellItem
	ret, _, _ = syscall.SyscallN(dialog.vtbl.GetResult,
		uintptr(unsafe.Pointer(dialog)),
		uintptr(unsafe.Pointer(&item)))
	if ret != 0 {
		return "", fmt.Errorf("GetResult failed")
	}
	defer syscall.SyscallN(item.vtbl.Release, uintptr(unsafe.Pointer(item)))

	// Get file path
	var pathPtr *uint16
	ret, _, _ = syscall.SyscallN(item.vtbl.GetDisplayName,
		uintptr(unsafe.Pointer(item)),
		SIGDN_FILESYSPATH,
		uintptr(unsafe.Pointer(&pathPtr)))
	if ret != 0 {
		return "", fmt.Errorf("GetDisplayName failed")
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(pathPtr)))
	path := windows.UTF16PtrToString(pathPtr)
	path = filepath.ToSlash(path)
	return path, nil
}

// func main() {
// 	folder, err := PickFileModern()
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println(folder)
// }
