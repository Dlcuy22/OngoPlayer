//go:build windows

package loader

import (
	"fmt"
	"syscall"
)

func Load(name string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(name)
	if err != nil {
		err = fmt.Errorf("%s: error loading library: %w", name, err)
	}
	return uintptr(handle), err
}

func LoadWithPath(name string) (uintptr, string, error) {
	handle, err := syscall.LoadLibrary(name)
	if err != nil {
		return 0, "", fmt.Errorf("%s: error loading library: %w", name, err)
	}
	return uintptr(handle), name, nil
}

func Get(lib uintptr, name string) uintptr {
	addr, err := syscall.GetProcAddress(syscall.Handle(lib), name)
	if err != nil {
		panic(err)
	}
	return addr
}

