package ogg

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// Packet holds a single Opus audio packet extracted from an OGG container.
type Packet struct {
	Data      []byte
	GranulePos uint64
}

// File holds parsed Opus packets from an OGG file.
type File struct {
	SampleRate uint32
	Channels   uint8
	Packets    []Packet
}

// ReadFile reads an OGG/Opus file and extracts all audio packets.
func ReadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ogg: read file: %w", err)
	}

	f := &File{SampleRate: 48000, Channels: 2}
	offset := 0
	pageNum := 0

	for offset+27 <= len(data) {
		// Check capture pattern
		if string(data[offset:offset+4]) != "OggS" {
			return nil, fmt.Errorf("ogg: invalid capture pattern at offset %d", offset)
		}

		// Parse page header
		numSegments := int(data[offset+26])
		if offset+27+numSegments > len(data) {
			break
		}

		granulePos := binary.LittleEndian.Uint64(data[offset+6 : offset+14])
		segmentTable := data[offset+27 : offset+27+numSegments]

		// Calculate total data size and extract packets
		dataOffset := offset + 27 + numSegments
		var packetData []byte

		for _, segLen := range segmentTable {
			end := dataOffset + int(segLen)
			if end > len(data) {
				break
			}
			packetData = append(packetData, data[dataOffset:end]...)
			dataOffset = end

			// A segment < 255 bytes terminates a packet
			if segLen < 255 && len(packetData) > 0 {
				// Skip the first two pages (OpusHead + OpusTags headers)
				if pageNum >= 2 || (pageNum < 2 && !isOpusHeader(packetData)) {
					f.Packets = append(f.Packets, Packet{
						Data:       append([]byte{}, packetData...),
						GranulePos: granulePos,
					})
				} else if pageNum == 0 && len(packetData) >= 19 {
					// Parse OpusHead to get sample rate and channels
					f.Channels = packetData[9]
					f.SampleRate = binary.LittleEndian.Uint32(packetData[12:16])
					if f.SampleRate == 0 {
						f.SampleRate = 48000
					}
				}
				packetData = nil
			}
		}

		// If there's remaining packet data (continued on next page)
		if len(packetData) > 0 && pageNum >= 2 {
			f.Packets = append(f.Packets, Packet{
				Data:       packetData,
				GranulePos: granulePos,
			})
		}

		offset = dataOffset
		pageNum++
	}

	if len(f.Packets) == 0 {
		return nil, fmt.Errorf("ogg: no audio packets found")
	}

	return f, nil
}

// FrameDuration returns the duration of a single Opus frame (typically 20ms).
func (f *File) FrameDuration() time.Duration {
	return 20 * time.Millisecond
}

// isOpusHeader checks if data starts with "OpusHead" or "OpusTags".
func isOpusHeader(data []byte) bool {
	if len(data) >= 8 {
		sig := string(data[:8])
		return sig == "OpusHead" || sig == "OpusTags"
	}
	return false
}
