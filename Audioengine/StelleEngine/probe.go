package stelleengine

import (
	"log/slog"
	"runtime"

	"github.com/dlcuy22/OngoPlayer/Audioengine/loader"
)

type codecLib struct {
	Name   string
	Format string
	Files  []string
}

var probedLibs = []codecLib{
	{Name: "libopusfile", Format: "Opus", Files: libName("libopusfile", "so.0", "0.dll", "0.dylib")},
	{Name: "libvorbisfile", Format: "Vorbis", Files: libName("libvorbisfile", "so.3", "3.dll", "dylib")},
	{Name: "libmpg123", Format: "MP3", Files: libName("libmpg123", "so.0", "0.dll", "dylib")},
	{Name: "libFLAC", Format: "FLAC", Files: libNameFLAC()},
	{Name: "stelle_dsp (C DSP)", Format: "DSP", Files: libNameDSP()},
}

func libName(base, linuxSuffix, winSuffix, darwinSuffix string) []string {
	switch runtime.GOOS {
	case "linux", "freebsd":
		return []string{base + "." + linuxSuffix}
	case "windows":
		return []string{base + "-" + winSuffix}
	case "darwin":
		return []string{base + "." + darwinSuffix}
	}
	return nil
}

func libNameFLAC() []string {
	switch runtime.GOOS {
	case "linux", "freebsd":
		return []string{"libFLAC.so.14", "libFLAC.so.12", "libFLAC.so"}
	case "windows":
		return []string{"libFLAC.dll", "libFLAC-14.dll", "libFLAC-12.dll"}
	case "darwin":
		return []string{"libFLAC.dylib"}
	}
	return nil
}

func libNameDSP() []string {
	switch runtime.GOOS {
	case "linux", "freebsd":
		return []string{"stelle_dsp.so"}
	case "windows":
		return []string{"stelle_dsp.dll"}
	case "darwin":
		return []string{"stelle_dsp.dylib"}
	}
	return nil
}

var probeLog = slog.With("sub", "probe")

func ProbeCodecs() {
	for _, lib := range probedLibs {
		ok := false
		for _, fn := range lib.Files {
			_, path, err := loader.LoadWithPath(fn)
			if err == nil {
				probeLog.Info("Loaded " + lib.Name, "path", path)
				ok = true
				break
			}
		}
		if !ok {
			probeLog.Warn("runtime library " + lib.Name + " not found, " + lib.Format + " audio format cannot be loaded")
		}
	}
}
