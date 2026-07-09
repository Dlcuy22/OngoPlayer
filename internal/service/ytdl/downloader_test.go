// Package ytdl unit tests.
//
// Key Functions:
//   - TestNewDownloader(): Asserts downloader creation.
//   - TestBinaryNames(): Asserts binary name mappings based on GOOS.
//
package ytdl

import (
	"runtime"
	"testing"
)

func TestNewDownloader(t *testing.T) {
	dl := NewDownloader("testbin")
	if dl == nil {
		t.Fatal("expected downloader to not be nil")
	}
	if dl.exeDir != "testbin" {
		t.Errorf("expected testbin, got %s", dl.exeDir)
	}
}

func TestBinaryNames(t *testing.T) {
	dl := NewDownloader("testbin")
	name := dl.getBinaryName("test")
	if runtime.GOOS == "windows" {
		if name != "test.exe" {
			t.Errorf("expected test.exe on Windows, got %s", name)
		}
	} else {
		if name != "test" {
			t.Errorf("expected test on non-Windows, got %s", name)
		}
	}
}
