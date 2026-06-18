//go:build windows

package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func init() {
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 4; i++ {
			dllDir := filepath.Join(dir, "build", "win")
			if _, errStat := os.Stat(dllDir); errStat == nil {
				if kernel32, err := syscall.LoadLibrary("kernel32.dll"); err == nil {
					defer syscall.FreeLibrary(kernel32)
					if proc, err := syscall.GetProcAddress(kernel32, "SetDllDirectoryW"); err == nil {
						if u16, err := syscall.UTF16PtrFromString(dllDir); err == nil {
							_, _, _ = syscall.SyscallN(proc, uintptr(unsafe.Pointer(u16)))
							break
						}
					}
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
}

func Load(name string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(name)
	if err != nil {
		err = fmt.Errorf("%s: error loading library: %w", name, err)
	}
	return uintptr(handle), err
}

func Get(lib uintptr, name string) uintptr {
	addr, err := syscall.GetProcAddress(syscall.Handle(lib), name)
	if err != nil {
		panic(err)
	}
	return addr
}

