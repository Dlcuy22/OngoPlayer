package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ucrt64RepoURL       = "https://repo.msys2.org/mingw/ucrt64/"
	flacReleaseAssetURL = "https://github.com/xiph/flac/releases/download/1.5.0/flac-1.5.0-win.zip"
)

var neededPackages = []string{
	"mingw-w64-ucrt-x86_64-opusfile-0.12-4-any.pkg.tar.zst",
	"mingw-w64-ucrt-x86_64-opus-1.6.1-1-any.pkg.tar.zst",
	"mingw-w64-ucrt-x86_64-libogg-1.3.6-1-any.pkg.tar.zst",
	"mingw-w64-ucrt-x86_64-sdl3-3.4.4-1-any.pkg.tar.zst",
	"mingw-w64-ucrt-x86_64-libvorbis-1.3.7-2-any.pkg.tar.zst",
	"mingw-w64-ucrt-x86_64-mpg123-1.33.4-1-any.pkg.tar.zst",
	"flac-1.5.0-win.zip",
}

var neededFiles = []string{
	"libFLAC.dll",
	"libopus-0.dll",
	"libmpg123-0.dll",
	"libopusfile-0.dll",
	"libvorbisfile-3.dll",
	"libogg-0.dll",
	"libvorbis-0.dll",
	"SDL3.dll",
}

func main() {
	keepDownloads := flag.Bool("keep-downloads", false, "Keep downloaded archives in build/.downloads")
	flag.Parse()

	buildDir := "./build/win"

	libsExtractDir := filepath.Join(buildDir, "libs")
	flacExtractDir := filepath.Join(buildDir, "lib")
	downloadDir := filepath.Join(buildDir, ".downloads")

	// Create directories
	for _, dir := range []string{buildDir, libsExtractDir, flacExtractDir, downloadDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	archives := make(map[string]string)

	// 1) Download all archives
	for _, pkg := range neededPackages {
		var url string
		if strings.HasSuffix(pkg, ".pkg.tar.zst") {
			url = ucrt64RepoURL + pkg
		} else if strings.HasSuffix(pkg, ".zip") && strings.HasPrefix(pkg, "flac-") {
			url = flacReleaseAssetURL
		} else {
			log.Fatalf("Unknown package mapping for: %s", pkg)
		}

		dest := filepath.Join(downloadDir, pkg)
		fmt.Printf("Downloading %s\n", url)
		if err := downloadFile(url, dest); err != nil {
			log.Fatalf("Failed to download %s: %v", url, err)
		}
		archives[pkg] = dest
	}

	// 2 & 3) Extract archives
	for pkg, archivePath := range archives {
		if strings.HasSuffix(pkg, ".pkg.tar.zst") {
			fmt.Printf("Extracting %s -> %s\n", pkg, libsExtractDir)
			if err := extractPkgTarZst(archivePath, libsExtractDir); err != nil {
				log.Fatalf("Failed to extract %s: %v", archivePath, err)
			}
		} else if strings.HasSuffix(pkg, ".zip") {
			fmt.Printf("Extracting %s -> %s\n", filepath.Base(archivePath), flacExtractDir)
			if err := safeExtractZip(archivePath, flacExtractDir); err != nil {
				log.Fatalf("Failed to extract %s: %v", archivePath, err)
			}
		}
	}

	// 4) Copy needed DLLs from ucrt64/bin into build/
	ucrtBin := filepath.Join(libsExtractDir, "ucrt64", "bin")
	if _, err := os.Stat(ucrtBin); os.IsNotExist(err) {
		log.Fatalf("Expected MSYS2 bin directory not found: %s", ucrtBin)
	}

	fmt.Println("Copying selected DLLs from ucrt64/bin into build/")
	for _, name := range neededFiles {
		src := filepath.Join(ucrtBin, name)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(buildDir, name)
			if err := copyFile(src, dst); err != nil {
				log.Fatalf("Failed to copy %s: %v", src, err)
			}
		}
	}

	// 5) Find and copy libFLAC.dll from the Win64 folder
	var flacDll string
	err := filepath.WalkDir(flacExtractDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "libFLAC.dll") {
			// Check if "win64" is in the path hierarchy
			parts := strings.Split(filepath.ToSlash(path), "/")
			for _, part := range parts {
				if strings.EqualFold(part, "win64") {
					flacDll = path
					return filepath.SkipAll // Stop searching once found
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking FLAC directory: %v", err)
	}
	if flacDll == "" {
		log.Fatalf("Could not find Win64 libFLAC.dll under %s", flacExtractDir)
	}

	fmt.Printf("Copying %s (Win64) -> %s\n", filepath.Base(flacDll), filepath.Join(buildDir, "libFLAC.dll"))
	if err := copyFile(flacDll, filepath.Join(buildDir, "libFLAC.dll")); err != nil {
		log.Fatalf("Failed to copy FLAC dll: %v", err)
	}

	// 6) Cleanup intermediate extraction directories
	fmt.Println("Cleaning up intermediate extraction directories...")
	os.RemoveAll(libsExtractDir)
	os.RemoveAll(flacExtractDir)

	// 7) Optionally remove downloads
	if !*keepDownloads {
		os.RemoveAll(downloadDir)
	}

	fmt.Println("Done. Required DLLs are staged in the build directory.")
}

// downloadFile performs a simple HTTP GET with a User-Agent and writes to dest.
func downloadFile(url, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad HTTP status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractPkgTarZst decompresses .zst using 'zstd' and extracts the .tar using 'tar'.
func extractPkgTarZst(archive, dest string) error {
	tmpTar := filepath.Join(filepath.Dir(dest), strings.TrimSuffix(filepath.Base(archive), ".zst"))

	// Step 1: zstd -d -f archive -o tmpTar
	cmdZstd := exec.Command("zstd", "-d", "-f", archive, "-o", tmpTar)
	if out, err := cmdZstd.CombinedOutput(); err != nil {
		return fmt.Errorf("zstd failed: %v\nOutput: %s", err, out)
	}
	defer os.Remove(tmpTar) // Cleanup intermediate tar

	// Step 2: tar -xf tmpTar -C dest
	cmdTar := exec.Command("tar", "-xf", tmpTar, "-C", dest)
	if out, err := cmdTar.CombinedOutput(); err != nil {
		return fmt.Errorf("tar failed: %v\nOutput: %s", err, out)
	}

	return nil
}

// safeExtractZip safely extracts a zip archive preventing directory traversal (ZipSlip).
func safeExtractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	for _, f := range r.File {
		fpath := filepath.Join(destAbs, f.Name)

		// Prevent ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(destAbs)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe file path detected: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// copyFile handles copying a file from src to dst.
func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
