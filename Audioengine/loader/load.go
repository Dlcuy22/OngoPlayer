//go:build !windows

package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
)

func Load(name string) (uintptr, error) {
	p, _, err := loadWithPath(name)
	return p, err
}

func LoadWithPath(name string) (uintptr, string, error) {
	return loadWithPath(name)
}

// loadWithPath returns the loaded library handle, the resolved file path, and any error.
func loadWithPath(name string) (uintptr, string, error) {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		exePath := filepath.Join(exeDir, name)
		if p, err := purego.Dlopen(exePath, purego.RTLD_LAZY); err == nil {
			return p, exePath, nil
		}
	}

	localPath := fmt.Sprintf(".%s%s", string(os.PathSeparator), name)
	if p, err := purego.Dlopen(localPath, purego.RTLD_LAZY); err == nil {
		if abs, err := filepath.Abs(localPath); err == nil {
			return p, abs, nil
		}
		return p, localPath, nil
	}

	if p, err := purego.Dlopen(name, purego.RTLD_LAZY); err == nil {
		return p, name, nil
	}
	return 0, "", fmt.Errorf("failed to load library: %s", name)
}

func Get(lib uintptr, name string) uintptr {
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		panic(err)
	}
	return addr
}
