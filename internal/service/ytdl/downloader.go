// Package ytdl provides capability to download, extract, and execute yt-dlp,
// FFmpeg/FFprobe, and Deno JS runtime within the application directory.
//
// Key Functions:
//   - NewDownloader(): Instantiates the downloader service.
//   - CheckAndInstall(): Downloads and extracts missing binaries dynamically.
//   - DownloadSong(): Spawns yt-dlp to download streams with correct quality/codec.
//
// Dependencies:
//   - archive/zip: Extracting zip archives.
//   - archive/tar: Extracting tar archives.
//   - github.com/ulikunitz/xz: Support extracting tar.xz on Linux.
//
// Error Types:
//   - ErrBinaryNotFound: Returned if binary path resolution fails.
//
// Example:
//   dl := ytdl.NewDownloader(exeDir)
//   err := dl.CheckAndInstall(ctx, func(name string, pct float64, done bool) {})
package ytdl

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ulikunitz/xz"
)

var ErrBinaryNotFound = errors.New("required binary not found")

type ProgressCallback func(name string, progress float64, done bool)

type Downloader struct {
	exeDir string
}

/*
NewDownloader creates a new Downloader service instance.

	params:
	      exeDir: directory containing the running executable where dependencies are saved
	returns:
	      *Downloader
*/
func NewDownloader(exeDir string) *Downloader {
	return &Downloader{exeDir: exeDir}
}

/*
CheckAndInstall verifies if yt-dlp, ffmpeg/ffprobe, and deno are installed, downloading them if missing.

	params:
	      ctx: execution context
	      progress: callback reporting download progress
	returns:
	      error
*/
func (d *Downloader) CheckAndInstall(ctx context.Context, progress ProgressCallback) error {
	if err := os.MkdirAll(d.exeDir, 0755); err != nil {
		return fmt.Errorf("unable to create bin directory: %w", err)
	}

	ytdlpPresent := d.isBinaryPresent(d.getBinaryName("yt-dlp"))
	ffmpegPresent := d.isBinaryPresent(d.getBinaryName("ffmpeg")) && d.isBinaryPresent(d.getBinaryName("ffprobe"))
	denoPresent := d.isBinaryPresent(d.getBinaryName("deno"))

	if !ytdlpPresent {
		if err := d.downloadYtdlp(ctx, progress); err != nil {
			return fmt.Errorf("failed to download yt-dlp: %w", err)
		}
	}
	if !ffmpegPresent {
		if err := d.downloadFfmpeg(ctx, progress); err != nil {
			return fmt.Errorf("failed to download ffmpeg/ffprobe: %w", err)
		}
	}
	if !denoPresent {
		if err := d.downloadDeno(ctx, progress); err != nil {
			return fmt.Errorf("failed to download deno: %w", err)
		}
	}

	return nil
}

/*
DownloadSong runs the local yt-dlp binary with quality and codec configurations.

	params:
	      ctx: execution context
	      songID: YouTube Music song identifier
	      destPath: destination file path to write output
	      quality: audio conversion quality (e.g. 0)
	      codec: audio output format (e.g. opus)
	returns:
	      error
*/
func (d *Downloader) DownloadSong(ctx context.Context, songID, destPath, quality, codec string) error {
	ytdlpName := d.getBinaryName("yt-dlp")
	ytdlpPath := filepath.Join(d.exeDir, ytdlpName)

	if !d.isBinaryPresent(ytdlpName) {
		return fmt.Errorf("yt-dlp binary not found: %w", ErrBinaryNotFound)
	}

	args := []string{
		"-f", "bestaudio",
		"-x",
		"--audio-format", codec,
		"--audio-quality", quality,
		"-o", destPath,
		"--no-playlist",
		"https://www.youtube.com/watch?v=" + songID,
	}

	cmd := exec.CommandContext(ctx, ytdlpPath, args...)

	// Prepend exeDir containing ffmpeg/ffprobe and deno to the command's PATH
	env := os.Environ()
	pathIndex := -1
	for i, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			pathIndex = i
			break
		}
	}

	pathVal := ""
	if pathIndex >= 0 {
		pathVal = env[pathIndex][5:]
	}

	newPath := d.exeDir + string(os.PathListSeparator) + pathVal
	if pathIndex >= 0 {
		env[pathIndex] = "PATH=" + newPath
	} else {
		env = append(env, "PATH="+newPath)
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yt-dlp download failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (d *Downloader) isBinaryPresent(name string) bool {
	path := filepath.Join(d.exeDir, name)
	_, err := os.Stat(path)
	return err == nil
}

func (d *Downloader) getBinaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

type progressWriter struct {
	w          io.Writer
	total      int64
	downloaded int64
	name       string
	callback   ProgressCallback
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.downloaded += int64(n)
	if pw.callback != nil && pw.total > 0 {
		pct := float64(pw.downloaded) / float64(pw.total) * 100.0
		pw.callback(pw.name, pct, false)
	}
	return n, err
}

func (d *Downloader) downloadFile(ctx context.Context, name, url string, progress ProgressCallback) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	tmpFile := filepath.Join(d.exeDir, name+".tmp")
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	pw := &progressWriter{
		w:        f,
		total:    resp.ContentLength,
		name:     name,
		callback: progress,
	}

	if _, err = io.Copy(pw, resp.Body); err != nil {
		_ = os.Remove(tmpFile)
		return "", err
	}

	return tmpFile, nil
}

func (d *Downloader) downloadYtdlp(ctx context.Context, progress ProgressCallback) error {
	var url string
	switch runtime.GOOS {
	case "windows":
		url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	case "darwin":
		url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"
	default:
		url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
	}

	name := d.getBinaryName("yt-dlp")
	tmpFile, err := d.downloadFile(ctx, "yt-dlp", url, progress)
	if err != nil {
		return err
	}

	destPath := filepath.Join(d.exeDir, name)
	_ = os.Remove(destPath)
	if err = os.Rename(tmpFile, destPath); err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(destPath, 0755)
	}

	if progress != nil {
		progress("yt-dlp", 100.0, true)
	}
	return nil
}

func (d *Downloader) downloadFfmpeg(ctx context.Context, progress ProgressCallback) error {
	var url string
	isZip := true

	switch runtime.GOOS {
	case "windows":
		url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	case "darwin":
		// macOS will download individual zip files. Let's do ffmpeg zip first.
		url = "https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip"
	default:
		url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
		isZip = false
	}

	tmpFile, err := d.downloadFile(ctx, "ffmpeg", url, progress)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	if isZip {
		if err = d.extractZipBinaries(tmpFile, []string{"ffmpeg", "ffprobe"}); err != nil {
			return err
		}
	} else {
		if err = d.extractTarXzBinaries(tmpFile, []string{"ffmpeg", "ffprobe"}); err != nil {
			return err
		}
	}

	// For macOS evermeet, ffprobe is a separate download.
	if runtime.GOOS == "darwin" {
		urlProbe := "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip"
		tmpProbe, errProbe := d.downloadFile(ctx, "ffprobe", urlProbe, progress)
		if errProbe != nil {
			return errProbe
		}
		defer os.Remove(tmpProbe)
		if err = d.extractZipBinaries(tmpProbe, []string{"ffprobe"}); err != nil {
			return err
		}
	}

	if progress != nil {
		progress("ffmpeg", 100.0, true)
	}
	return nil
}

func (d *Downloader) downloadDeno(ctx context.Context, progress ProgressCallback) error {
	var target string
	arch := runtime.GOARCH
	osName := runtime.GOOS

	if osName == "windows" {
		if arch == "amd64" {
			target = "x86_64-pc-windows-msvc"
		} else if arch == "arm64" {
			target = "aarch64-pc-windows-msvc"
		}
	} else if osName == "linux" {
		if arch == "amd64" {
			target = "x86_64-unknown-linux-gnu"
		} else if arch == "arm64" {
			target = "aarch64-unknown-linux-gnu"
		}
	} else if osName == "darwin" {
		if arch == "amd64" {
			target = "x86_64-apple-darwin"
		} else if arch == "arm64" {
			target = "aarch64-apple-darwin"
		}
	}

	if target == "" {
		return fmt.Errorf("unsupported arch/OS for deno: %s/%s", osName, arch)
	}

	url := "https://github.com/denoland/deno/releases/latest/download/deno-" + target + ".zip"
	tmpFile, err := d.downloadFile(ctx, "deno", url, progress)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	if err = d.extractZipBinaries(tmpFile, []string{"deno"}); err != nil {
		return err
	}

	if progress != nil {
		progress("deno", 100.0, true)
	}
	return nil
}

func (d *Downloader) extractZipBinaries(archivePath string, binaries []string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		baseName := filepath.Base(f.Name)
		cleanBaseName := strings.TrimSuffix(baseName, ".exe")

		matched := false
		for _, b := range binaries {
			if cleanBaseName == b {
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		destPath := filepath.Join(d.exeDir, baseName)
		if err = d.writeFileFromZipFile(f, destPath); err != nil {
			return err
		}
	}

	return nil
}

func (d *Downloader) writeFileFromZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_ = os.Remove(destPath)
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func (d *Downloader) extractTarXzBinaries(archivePath string, binaries []string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	xzReader, err := xz.NewReader(bufio.NewReader(file))
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(xzReader)
	for {
		header, errTar := tarReader.Next()
		if errors.Is(errTar, io.EOF) {
			break
		}
		if errTar != nil {
			return errTar
		}

		baseName := filepath.Base(header.Name)
		matched := false
		for _, b := range binaries {
			if baseName == b {
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		destPath := filepath.Join(d.exeDir, baseName)
		_ = os.Remove(destPath)
		out, errOut := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if errOut != nil {
			return errOut
		}

		if _, err = io.Copy(out, tarReader); err != nil {
			out.Close()
			return err
		}
		out.Close()
	}

	return nil
}
