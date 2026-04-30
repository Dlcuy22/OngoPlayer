package main

import (
	"bytes"
	"context"
	"debug/pe"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Win32 Constants
const (
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
	LIST_MODULES_ALL          = 0x03
	ERROR_PARTIAL_COPY        = 299
)

// Win32 Structs
type FILETIME struct {
	DwLowDateTime  uint32
	DwHighDateTime uint32
}

type PROCESS_MEMORY_COUNTERS_EX struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

// Global API Pointers
var (
	openProcess          func(dwDesiredAccess uint32, bInheritHandle bool, dwProcessId uint32) uintptr
	closeHandle          func(hObject uintptr) bool
	getProcessTimes      func(hProcess uintptr, lpCreationTime, lpExitTime, lpKernelTime, lpUserTime *FILETIME) bool
	enumProcessModulesEx func(hProcess uintptr, lphModule *uintptr, cb uint32, lpcbNeeded *uint32, dwFilterFlag uint32) bool
	getModuleFileNameExW func(hProcess uintptr, hModule uintptr, lpFilename *uint16, nSize uint32) uint32
	getProcessMemoryInfo func(Process uintptr, ppsmemCounters *PROCESS_MEMORY_COUNTERS_EX, cb uint32) bool
)

func init() {
	// purego.Dlopen is POSIX-only. On Windows, we use standard syscall.LoadLibrary.
	kernel32, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		panic(fmt.Errorf("failed to load kernel32.dll: %v", err))
	}
	psapi, err := syscall.LoadLibrary("psapi.dll")
	if err != nil {
		panic(fmt.Errorf("failed to load psapi.dll: %v", err))
	}

	// RegisterLibFunc expects (function_pointer, uintptr_handle, func_name_string)
	purego.RegisterLibFunc(&openProcess, uintptr(kernel32), "OpenProcess")
	purego.RegisterLibFunc(&closeHandle, uintptr(kernel32), "CloseHandle")
	purego.RegisterLibFunc(&getProcessTimes, uintptr(kernel32), "GetProcessTimes")
	purego.RegisterLibFunc(&enumProcessModulesEx, uintptr(psapi), "EnumProcessModulesEx")
	purego.RegisterLibFunc(&getModuleFileNameExW, uintptr(psapi), "GetModuleFileNameExW")
	purego.RegisterLibFunc(&getProcessMemoryInfo, uintptr(psapi), "GetProcessMemoryInfo")
}

func filetimeToInt(ft FILETIME) uint64 {
	return (uint64(ft.DwHighDateTime) << 32) | uint64(ft.DwLowDateTime)
}

func getProcessTimesSeconds(hProcess uintptr) float64 {
	var c, e, k, u FILETIME
	if !getProcessTimes(hProcess, &c, &e, &k, &u) {
		return 0.0
	}
	total100ns := filetimeToInt(k) + filetimeToInt(u)
	return float64(total100ns) / 10000000.0
}

func getProcessMemoryMiB(hProcess uintptr) (float64, float64) {
	var counters PROCESS_MEMORY_COUNTERS_EX
	counters.Cb = uint32(unsafe.Sizeof(counters))
	if !getProcessMemoryInfo(hProcess, &counters, counters.Cb) {
		return 0.0, 0.0
	}
	workingSet := float64(counters.WorkingSetSize) / (1024 * 1024)
	privateUsage := float64(counters.PrivateUsage) / (1024 * 1024)
	return workingSet, privateUsage
}

func listLoadedModules(pid uint32) (map[string]bool, error) {
	hProcess := openProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, pid)
	if hProcess == 0 {
		return nil, syscall.GetLastError()
	}
	defer closeHandle(hProcess)

	capacity := uint32(1024)
	for {
		modules := make([]uintptr, capacity)
		var needed uint32

		if !enumProcessModulesEx(hProcess, &modules[0], capacity*uint32(unsafe.Sizeof(uintptr(0))), &needed, LIST_MODULES_ALL) {
			return nil, syscall.GetLastError()
		}

		count := needed / uint32(unsafe.Sizeof(uintptr(0)))
		if count > capacity {
			capacity = count + 32
			continue
		}

		result := make(map[string]bool)
		for i := uint32(0); i < count; i++ {
			hMod := modules[i]
			buf := make([]uint16, 4096)
			length := getModuleFileNameExW(hProcess, hMod, &buf[0], uint32(len(buf)))
			if length > 0 {
				path := syscall.UTF16ToString(buf[:length])
				path = strings.ToLower(filepath.Clean(path))
				result[path] = true
			}
		}
		return result, nil
	}
}

func tryGoVersionM(path string) string {
	// Fixed context initialization
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func analyzeExecutable(exePath string) {
	fmt.Println("[EXECUTABLE ANALYSIS]")
	fmt.Printf("Path: %s\n", exePath)

	peFile, err := pe.Open(exePath)
	if err != nil {
		fmt.Println("Not a valid PE file or could not parse headers.")
		return
	}
	defer peFile.Close()

	var arch string
	var imageBase uint64
	var subsystem uint16
	var entryPoint uint32

	switch oh := peFile.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		arch = "x86 (PE32)"
		imageBase = uint64(oh.ImageBase)
		subsystem = oh.Subsystem
		entryPoint = oh.AddressOfEntryPoint
	case *pe.OptionalHeader64:
		arch = "x64 (PE32+)"
		imageBase = oh.ImageBase
		subsystem = oh.Subsystem
		entryPoint = oh.AddressOfEntryPoint
	default:
		arch = "unknown"
	}

	subsystemMap := map[uint16]string{
		1: "Native", 2: "Windows GUI", 3: "Windows CUI", 5: "OS/2 CUI",
		7: "POSIX CUI", 9: "Windows CE GUI", 10: "EFI Application",
		11: "EFI Boot Service Driver", 12: "EFI Runtime Driver",
		13: "EFI ROM", 14: "Xbox", 16: "Windows Boot Application",
	}
	subName, ok := subsystemMap[subsystem]
	if !ok {
		subName = fmt.Sprintf("unknown (%d)", subsystem)
	}

	fmt.Printf("Architecture: %s\n", arch)
	fmt.Printf("Subsystem: %s\n", subName)
	fmt.Printf("Entry point RVA: 0x%08x\n", entryPoint)
	fmt.Printf("Image base: 0x%x\n", imageBase)

	fmt.Println("Sections:")
	goSectionMarkers := []string{".gopclntab", ".go.buildinfo", ".noptrdata", ".noptrbss", ".note.go.buildid", ".zdebug_abbrev"}
	goSignals := []string{}

	for _, sec := range peFile.Sections {
		fmt.Printf("  %s (VA=0x%08x, VSZ=0x%08x, RAW=0x%08x)\n", sec.Name, sec.VirtualAddress, sec.VirtualSize, sec.Offset)
		lowerName := strings.ToLower(sec.Name)
		for _, m := range goSectionMarkers {
			if strings.Contains(lowerName, m) {
				goSignals = append(goSignals, fmt.Sprintf("section %s", m))
			}
		}
	}

	imports, _ := peFile.ImportedLibraries()
	if len(imports) > 0 {
		fmt.Println("Imports:")
		for _, dll := range imports {
			fmt.Printf("  %s\n", strings.ToLower(dll))
		}
	} else {
		fmt.Println("Imports: none found or import table not parsed")
	}

	goM := tryGoVersionM(exePath)
	if goM != "" {
		fmt.Println("go version -m output:")
		for _, line := range strings.Split(goM, "\n") {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println("Go detection: yes (go tool confirmed module metadata)")
		return
	}

	// Read file for strings search
	data, _ := os.ReadFile(exePath)
	markers := []string{"Go build ID:", "go1.", "runtime.main", "runtime.rt0_", "go.buildinfo", "Go buildinf:"}
	stringHits := []string{}

	// Simplified ASCII strings search
	var buf bytes.Buffer
	for _, b := range data {
		if b >= 32 && b <= 126 {
			buf.WriteByte(b)
		} else {
			if buf.Len() >= 6 {
				str := buf.String()
				lowerStr := strings.ToLower(str)
				for _, m := range markers {
					if strings.Contains(lowerStr, strings.ToLower(m)) {
						stringHits = append(stringHits, str)
						break
					}
				}
			}
			buf.Reset()
		}
	}

	if len(stringHits) > 0 {
		goSignals = append(goSignals, "strings contain Go runtime/build markers")
	}

	if len(goSignals) > 0 {
		fmt.Println("Go detection: likely")
		for _, s := range goSignals {
			fmt.Printf("  - %s\n", s)
		}
		if len(stringHits) > 0 {
			fmt.Println("  String hits:")
			limit := 5
			if len(stringHits) < 5 {
				limit = len(stringHits)
			}
			for _, h := range stringHits[:limit] {
				fmt.Printf("    %s\n", h)
			}
		}
	} else {
		fmt.Println("Go detection: not obvious from PE heuristics")
	}
}

type MetricStats struct {
	samples uint
	sumCPU  float64
	maxCPU  float64
	minCPU  float64
	sumWS   float64
	maxWS   float64
	minWS   float64
}

func (m *MetricStats) add(cpuPct, wsMiB float64) {
	if m.samples == 0 {
		m.maxCPU, m.minCPU = cpuPct, cpuPct
		m.maxWS, m.minWS = wsMiB, wsMiB
	} else {
		m.maxCPU = math.Max(m.maxCPU, cpuPct)
		m.minCPU = math.Min(m.minCPU, cpuPct)
		m.maxWS = math.Max(m.maxWS, wsMiB)
		m.minWS = math.Min(m.minWS, wsMiB)
	}
	m.sumCPU += cpuPct
	m.sumWS += wsMiB
	m.samples++
}

func (m *MetricStats) avgCPU() float64 {
	if m.samples == 0 {
		return 0
	}
	return m.sumCPU / float64(m.samples)
}

func (m *MetricStats) avgWS() float64 {
	if m.samples == 0 {
		return 0
	}
	return m.sumWS / float64(m.samples)
}

func fmtStat(samples uint, val float64) string {
	if samples == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", val)
}

func main() {
	interval := flag.Float64("interval", 1.0, "Polling interval in seconds")
	startupDelay := flag.Float64("startup-delay", 1.0, "Delay after launch before monitoring")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <exe> [-- args...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	exePath, err := filepath.Abs(args[0])
	if err != nil || func() bool { _, e := os.Stat(exePath); return os.IsNotExist(e) }() {
		fmt.Fprintf(os.Stderr, "Executable not found: %s\n", exePath)
		os.Exit(1)
	}

	var childArgs []string
	if len(args) > 1 {
		if args[1] == "--" {
			childArgs = args[2:]
		} else {
			childArgs = args[1:]
		}
	}

	analyzeExecutable(exePath)
	fmt.Println()

	cmd := exec.Command(exePath, childArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Starting: %v\n", cmd.Args)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start process: %v\n", err)
		os.Exit(1)
	}

	pid := uint32(cmd.Process.Pid)
	fmt.Printf("PID: %d\n", pid)
	fmt.Println("Watching loaded DLLs and process metrics...\n")

	time.Sleep(time.Duration(*startupDelay * float64(time.Second)))

	seenModules := make(map[string]bool)
	stats := &MetricStats{}

	hProcess := openProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, pid)
	if hProcess == 0 {
		fmt.Fprintf(os.Stderr, "Failed to open process %d: %v\n", pid, syscall.GetLastError())
		os.Exit(1)
	}
	defer closeHandle(hProcess)

	prevCPUTime := getProcessTimesSeconds(hProcess)
	prevT := time.Now()

	// Handle Graceful Exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		cmd.Wait()
		close(done)
	}()

	ticker := time.NewTicker(time.Duration(*interval * float64(time.Second)))
	defer ticker.Stop()

	for {
		select {
		case <-done:
			fmt.Println("Process exited.")
			os.Exit(cmd.ProcessState.ExitCode())
		case <-sigChan:
			fmt.Println("\nStopping monitor...")
			cmd.Process.Kill()
			os.Exit(1)
		case <-ticker.C:
			currentModules, err := listLoadedModules(pid)
			if err != nil {
				// Handle ERROR_PARTIAL_COPY (299)
				if err.(syscall.Errno) == ERROR_PARTIAL_COPY {
					currentModules = make(map[string]bool)
				} else {
					fmt.Fprintf(os.Stderr, "Error listing modules: %v\n", err)
					continue
				}
			}

			nowT := time.Now()
			nowCPUTime := getProcessTimesSeconds(hProcess)

			elapsed := nowT.Sub(prevT).Seconds()
			if elapsed <= 0 {
				elapsed = 1e-6
			}

			cpuDelta := nowCPUTime - prevCPUTime
			if cpuDelta < 0 {
				cpuDelta = 0
			}

			// Note: This calculates CPU % of one core.
			cpuPct := (cpuDelta / elapsed) * 100.0
			workingSetMiB, privateMiB := getProcessMemoryMiB(hProcess)

			stats.add(cpuPct, workingSetMiB)

			for dll := range currentModules {
				if !seenModules[dll] {
					fmt.Printf("[+ DLL] %s\n", dll)
				}
			}

			fmt.Printf("[CPU] current=%.1f%% avg=%.1f%% max=%s%% low=%s%%\n",
				cpuPct, stats.avgCPU(), fmtStat(stats.samples, stats.maxCPU), fmtStat(stats.samples, stats.minCPU))
			fmt.Printf("[RAM] working_set=%.1f MiB private=%.1f MiB avg_ws=%.1f MiB max_ws=%s MiB low_ws=%s MiB\n\n",
				workingSetMiB, privateMiB, stats.avgWS(), fmtStat(stats.samples, stats.maxWS), fmtStat(stats.samples, stats.minWS))

			seenModules = currentModules
			prevCPUTime = nowCPUTime
			prevT = nowT
		}
	}
}
