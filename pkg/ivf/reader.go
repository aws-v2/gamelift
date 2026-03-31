package ivf

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// Frame holds a single decoded IVF video frame.
type Frame struct {
	Data      []byte
	Timestamp uint64
}

// File holds the parsed contents of an IVF container.
type File struct {
	Codec     string
	Width     uint16
	Height    uint16
	TimebaseN uint32 // numerator
	TimebaseD uint32 // denominator
	Frames    []Frame
}

// ReadFile reads an entire IVF file into memory.
func ReadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ivf: read file: %w", err)
	}
	if len(data) < 32 {
		return nil, fmt.Errorf("ivf: file too small for header")
	}

	sig := string(data[0:4])
	if sig != "DKIF" {
		return nil, fmt.Errorf("ivf: invalid signature: %s", sig)
	}

	f := &File{
		Codec:     string(data[8:12]),
		Width:     binary.LittleEndian.Uint16(data[12:14]),
		Height:    binary.LittleEndian.Uint16(data[14:16]),
		TimebaseD: binary.LittleEndian.Uint32(data[16:20]),
		TimebaseN: binary.LittleEndian.Uint32(data[20:24]),
	}

	offset := 32
	for offset+12 <= len(data) {
		frameSize := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		timestamp := binary.LittleEndian.Uint64(data[offset+4 : offset+12])
		offset += 12

		if offset+frameSize > len(data) {
			break
		}

		frameData := make([]byte, frameSize)
		copy(frameData, data[offset:offset+frameSize])

		f.Frames = append(f.Frames, Frame{
			Data:      frameData,
			Timestamp: timestamp,
		})
		offset += frameSize
	}

	if len(f.Frames) == 0 {
		return nil, fmt.Errorf("ivf: no frames found")
	}

	return f, nil
}

// FrameDuration estimates the average duration of a single frame from timestamps.
func (f *File) FrameDuration() time.Duration {
	if len(f.Frames) < 2 || f.TimebaseD == 0 {
		return 33 * time.Millisecond // fallback 30fps
	}
	firstTS := f.Frames[0].Timestamp
	lastTS := f.Frames[len(f.Frames)-1].Timestamp
	if lastTS <= firstTS {
		return 33 * time.Millisecond
	}
	totalSec := float64(lastTS-firstTS) / float64(f.TimebaseD)
	avg := time.Duration(totalSec / float64(len(f.Frames)-1) * float64(time.Second))
	if avg < 5*time.Millisecond || avg > time.Second {
		return 33 * time.Millisecond
	}
	return avg
}

// TimestampDuration converts a timestamp difference to a time.Duration.
func (f *File) TimestampDuration(tsDiff uint64) time.Duration {
	if f.TimebaseD == 0 {
		return 33 * time.Millisecond
	}
	dur := time.Duration(float64(tsDiff) / float64(f.TimebaseD) * float64(time.Second))
	if dur < 5*time.Millisecond || dur > time.Second {
		return f.FrameDuration()
	}
	return dur
}
