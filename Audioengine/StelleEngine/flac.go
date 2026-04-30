// Audioengine/StelleEngine/flac.go
// FLAC decoder implementation using libFLAC via purego.
// Provides FlacChunkDecoder which implements ChunkDecoder and SeekableChunkDecoder.
//
// Dependencies:
//   - internal/shared: cross-platform dynamic library loading
//   - purego: register C function symbols without cgo
//
// Runtime Libraries:
//   - Linux:   libFLAC.so.14, libFLAC.so.12, libFLAC.so
//   - Windows: libFLAC.dll, libFLAC-14.dll, libFLAC-12.dll
//   - macOS:   libFLAC.dylib

package stelleengine

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dlcuy22/OngoPlayer/internal/shared"
	"github.com/ebitengine/purego"
)

var (
	flacOnce    sync.Once
	flacInitErr error

	flac_new            func() uintptr
	flac_init_file      func(decoder uintptr, filename *byte, write_cb uintptr, metadata_cb uintptr, err_cb uintptr, client_data uintptr) uint32
	flac_process_meta   func(decoder uintptr) int32
	flac_process_single func(decoder uintptr) int32
	flac_seek_absolute  func(decoder uintptr, sample int64) int32
	flac_delete         func(decoder uintptr)
	flac_finish         func(decoder uintptr) int32

	writeCbHandle    uintptr
	metadataCbHandle uintptr
	errorCbHandle    uintptr

	flacInstances sync.Map // int32 -> *FlacChunkDecoder
	flacIDCounter atomic.Int32
)

/*
initFlacBindings loads libFLAC and registers required C symbols.
Called once via sync.Once on first use.

	returns:
	      error: loading or registration errors
*/
func initFlacBindings() error {
	flacOnce.Do(func() {
		var filenames []string
		switch runtime.GOOS {
		case "linux", "freebsd":
			filenames = []string{"libFLAC.so.14", "libFLAC.so.12", "libFLAC.so.8", "libFLAC.so"}
		case "windows":
			filenames = []string{"libFLAC.dll", "libFLAC-14.dll", "libFLAC-12.dll", "libFLAC-8.dll"}
		case "darwin":
			filenames = []string{"libFLAC.dylib"}
		}

		var lib uintptr
		var err error
		for _, fn := range filenames {
			lib, err = shared.Load(fn)
			if err == nil {
				fmt.Printf("Loaded FLAC library: %s\n", fn)
				break
			} else {
				fmt.Printf("Failed to load FLAC library: %s (%v)\n", fn, err)
			}
		}

		if lib == 0 {
			flacInitErr = fmt.Errorf("failed to load libFLAC")
			return
		}

		purego.RegisterLibFunc(&flac_new, lib, "FLAC__stream_decoder_new")
		purego.RegisterLibFunc(&flac_init_file, lib, "FLAC__stream_decoder_init_file")
		purego.RegisterLibFunc(&flac_process_meta, lib, "FLAC__stream_decoder_process_until_end_of_metadata")
		purego.RegisterLibFunc(&flac_process_single, lib, "FLAC__stream_decoder_process_single")
		purego.RegisterLibFunc(&flac_seek_absolute, lib, "FLAC__stream_decoder_seek_absolute")
		purego.RegisterLibFunc(&flac_delete, lib, "FLAC__stream_decoder_delete")
		purego.RegisterLibFunc(&flac_finish, lib, "FLAC__stream_decoder_finish")

		writeCbHandle = purego.NewCallback(flacWriteCallback)
		metadataCbHandle = purego.NewCallback(flacMetadataCallback)
		errorCbHandle = purego.NewCallback(flacErrorCallback)
	})
	return flacInitErr
}

/*
flacWriteCallback is called by libFLAC when audio samples are decoded.
It converts internal int32 samples to float32 and interleaves them.
*/
func flacWriteCallback(decoder uintptr, frame uintptr, buffer uintptr, client_data uintptr) uintptr {
	id := int32(client_data)
	cdIntf, ok := flacInstances.Load(id)
	if !ok {
		return 1 // FLAC__STREAM_DECODER_WRITE_STATUS_ABORT
	}
	cd := cdIntf.(*FlacChunkDecoder)

	blocksize := *(*uint32)(unsafe.Pointer(frame))
	channels := *(*uint32)(unsafe.Pointer(frame + 8))
	bitsPerSample := *(*uint32)(unsafe.Pointer(frame + 16))

	if bitsPerSample == 0 {
		bitsPerSample = uint32(cd.bitsPerSample)
	}

	if blocksize > 0 && channels > 0 {
		bufPtrs := unsafe.Slice((**int32)(unsafe.Pointer(buffer)), channels)

		needed := int(blocksize * channels)
		if len(cd.stagingBuf) < needed {
			cd.stagingBuf = make([]float32, needed)
		}

		// Support 16, 24, 32 bit integers
		divisor := float32((int64(1) << (bitsPerSample - 1)) - 1)
		if divisor <= 0 {
			divisor = 32767.0
		}

		for ch := 0; ch < int(channels); ch++ {
			chData := unsafe.Slice(bufPtrs[ch], blocksize)
			for i := 0; i < int(blocksize); i++ {
				cd.stagingBuf[i*int(channels)+ch] = float32(chData[i]) / divisor
			}
		}

		cd.stagePos = 0
		cd.stageLen = needed
	}

	return 0 // FLAC__STREAM_DECODER_WRITE_STATUS_CONTINUE
}

/*
flacMetadataCallback is called by libFLAC when metadata blocks are encountered.
Used to extract STREAMINFO (sample rate, channels, bit depth, total samples).
*/
func flacMetadataCallback(decoder uintptr, metadata uintptr, client_data uintptr) uintptr {
	id := int32(client_data)
	cdIntf, ok := flacInstances.Load(id)
	if !ok {
		return 0
	}
	cd := cdIntf.(*FlacChunkDecoder)

	metaType := *(*uint32)(unsafe.Pointer(metadata))
	if metaType == 0 { // FLAC__METADATA_TYPE_STREAMINFO
		ptrSize := unsafe.Sizeof(uintptr(0))
		unionOffset := uintptr(12)
		if ptrSize == 8 {
			unionOffset = 16
		}

		streamInfoBase := metadata + unionOffset
		cd.sampleRate = int(*(*uint32)(unsafe.Pointer(streamInfoBase + 16)))
		cd.channels = int(*(*uint32)(unsafe.Pointer(streamInfoBase + 20)))
		cd.bitsPerSample = int(*(*uint32)(unsafe.Pointer(streamInfoBase + 24)))

		tsOffset := uintptr(28)
		if unsafe.Alignof(uint64(0)) == 8 {
			tsOffset = 32
		}
		cd.totalFrames = int64(*(*uint64)(unsafe.Pointer(streamInfoBase + tsOffset)))
	}
	return 0
}

/*
flacErrorCallback handles decoder errors reported by libFLAC.
*/
func flacErrorCallback(decoder uintptr, status uint32, client_data uintptr) uintptr {
	// Ignore errors, handled in process loops if critical
	return 0
}

type FlacDecoder struct{}

/*
NewFlacDecoder returns a new FLAC decoder factory.

	returns:
	      *FlacDecoder
*/
func NewFlacDecoder() *FlacDecoder {
	return &FlacDecoder{}
}

func (d *FlacDecoder) Name() string { return "flac" }

func (d *FlacDecoder) CanHandle(ext string) bool {
	return strings.ToLower(ext) == ".flac"
}

/*
isFlacFile checks if the given path has a FLAC extension.

	params:
	      path: file system path
	returns:
	      bool
*/
func isFlacFile(path string) bool {
	ext := filepath.Ext(path)
	decoder := NewFlacDecoder()
	return decoder.CanHandle(ext)
}

type FlacChunkDecoder struct {
	id            int32
	dh            uintptr
	channels      int
	sampleRate    int
	bitsPerSample int
	totalFrames   int64
	stagingBuf    []float32
	stagePos      int
	stageLen      int
}

/*
openFlacChunkDecoder returns an openFn for creating FLAC chunk decoders.

	params:
	      path: path to the .flac file
	returns:
	      openFn
*/
func openFlacChunkDecoder(path string) openFn {
	return func(seekTo float64) (ChunkDecoder, error) {
		if err := initFlacBindings(); err != nil {
			return nil, err
		}

		dh := flac_new()
		if dh == 0 {
			return nil, fmt.Errorf("FLAC__stream_decoder_new failed")
		}

		id := flacIDCounter.Add(1)
		cd := &FlacChunkDecoder{
			id: id,
			dh: dh,
		}
		flacInstances.Store(id, cd)

		cPath := append([]byte(path), 0)
		res := flac_init_file(dh, &cPath[0], writeCbHandle, metadataCbHandle, errorCbHandle, uintptr(id))
		if res != 0 { // FLAC__STREAM_DECODER_INIT_STATUS_OK == 0
			flacInstances.Delete(id)
			flac_delete(dh)
			return nil, fmt.Errorf("FLAC__stream_decoder_init_file failed: %d", res)
		}

		if res := flac_process_meta(dh); res == 0 {
			flacInstances.Delete(id)
			flac_delete(dh)
			return nil, fmt.Errorf("FLAC__stream_decoder_process_until_end_of_metadata failed")
		}

		if seekTo > 0 {
			targetFrame := int64(seekTo * float64(cd.sampleRate))
			if err := cd.SeekToFrame(targetFrame); err != nil {
				flacInstances.Delete(id)
				flac_delete(dh)
				return nil, err
			}
		}

		return cd, nil
	}
}

/*
ReadSamples decodes audio from the FLAC stream into the provided buffer.
Handles resampling if the source rate differs from the engine's output rate.

	params:
	      buf: destination buffer for float32 interleaved samples
	returns:
	      int: number of samples written
	      error: io.EOF or decoding errors
*/
func (c *FlacChunkDecoder) ReadSamples(buf []float32) (int, error) {
	// Accumulate raw decoded samples into a temporary slice first,
	// then resample and copy into buf. This avoids the bug where
	// Resample returns a new slice that never gets written back.
	var raw []float32
	needRaw := len(buf)

	// If resampling is needed (e.g. 96kHz -> 48kHz), we need more
	// raw samples to produce enough output after downsampling.
	if c.sampleRate != DefaultSampleRate && c.sampleRate > 0 && DefaultSampleRate > 0 {
		ratio := float64(c.sampleRate) / float64(DefaultSampleRate)
		needRaw = int(float64(len(buf))*ratio) + c.channels
	}

	raw = make([]float32, 0, needRaw)
	var lastErr error

	for len(raw) < needRaw {
		if c.stagePos < c.stageLen {
			avail := c.stageLen - c.stagePos
			space := needRaw - len(raw)
			toCopy := avail
			if space < avail {
				toCopy = space
			}
			raw = append(raw, c.stagingBuf[c.stagePos:c.stagePos+toCopy]...)
			c.stagePos += toCopy
		} else {
			// Reset staging state before calling process_single
			// so we can detect if the callback actually produced data
			c.stageLen = 0
			c.stagePos = 0

			if res := flac_process_single(c.dh); res == 0 {
				lastErr = io.EOF
				break
			}
			if c.stageLen == 0 {
				lastErr = io.EOF
				break
			}
		}
	}

	if len(raw) == 0 {
		return 0, io.EOF
	}

	// Resample if the file's native rate differs from the engine output rate
	output := raw
	if c.sampleRate != DefaultSampleRate && c.sampleRate > 0 && DefaultSampleRate > 0 {
		output = Resample(raw, c.sampleRate, DefaultSampleRate, c.channels)
	}

	n := copy(buf, output)
	return n, lastErr
}

/*
SeekToFrame moves the playback cursor to a specific sample frame.

	params:
	      frame: target frame offset
	returns:
	      error
*/
func (c *FlacChunkDecoder) SeekToFrame(frame int64) error {
	c.stagePos = 0
	c.stageLen = 0
	res := flac_seek_absolute(c.dh, frame)
	if res == 0 {
		return fmt.Errorf("FLAC__stream_decoder_seek_absolute failed")
	}
	return nil
}

/*
Channels returns the number of audio channels.
*/
func (c *FlacChunkDecoder) Channels() int { return c.channels }

/*
SampleRate returns the native sample rate of the file.
*/
func (c *FlacChunkDecoder) SampleRate() int { return c.sampleRate }

/*
TotalFrames returns the total number of audio frames.
*/
func (c *FlacChunkDecoder) TotalFrames() int64 { return c.totalFrames }

/*
Close releases all resources associated with the decoder.

	returns:
	      error
*/
func (c *FlacChunkDecoder) Close() error {
	flacInstances.Delete(c.id)
	flac_finish(c.dh)
	flac_delete(c.dh)
	return nil
}
