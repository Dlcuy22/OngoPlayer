// Audioengine/StelleEngine/ringbuffer_test.go
// Concurrency tests for the lock-based ring buffer. Run with -race.

package stelleengine

import (
	"sync"
	"testing"
)

func TestRingBufferWriteRead(t *testing.T) {
	rb := NewRingBuffer(4, 1) // cap = 4 float32
	if !rb.Write([]float32{1, 2, 3}) {
		t.Fatal("write returned false")
	}
	dst := make([]float32, 3)
	if n := rb.Read(dst); n != 3 {
		t.Fatalf("read n = %d, want 3", n)
	}
	if dst[0] != 1 || dst[1] != 2 || dst[2] != 3 {
		t.Errorf("dst = %v, want [1 2 3]", dst)
	}
}

func TestRingBufferCloseUnblocksWriter(t *testing.T) {
	rb := NewRingBuffer(2, 1) // cap = 2

	done := make(chan bool, 1)
	go func() {
		// Writing 4 values into a cap-2 buffer blocks until Close.
		ok := rb.Write([]float32{1, 2, 3, 4})
		done <- ok
	}()

	rb.Close()
	if ok := <-done; ok {
		t.Error("write should return false after Close")
	}
}

func TestRingBufferConcurrentRace(t *testing.T) {
	rb := NewRingBuffer(DefaultSampleRate, DefaultChannels)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		chunk := make([]float32, 256)
		for i := 0; i < 200; i++ {
			if !rb.Write(chunk) {
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		dst := make([]float32, 256)
		for i := 0; i < 200; i++ {
			rb.Read(dst)
			rb.Available()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			rb.Clear()
		}
	}()

	// Close after the workers spin up so writers/readers exit cleanly.
	rb.Close()
	wg.Wait()
}
