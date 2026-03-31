package vp8

import "encoding/binary"

// boolEncoder implements the VP8 boolean arithmetic encoder per RFC 6386 §7.
type boolEncoder struct {
	output   []byte
	range_   uint32
	bottom   uint32
	bitCount int
}

func newBoolEncoder() *boolEncoder {
	return &boolEncoder{range_: 255}
}

func (e *boolEncoder) writeBool(value bool, prob uint8) {
	split := 1 + (((e.range_ - 1) * uint32(prob)) >> 8)
	if value {
		e.bottom += split
		e.range_ -= split
	} else {
		e.range_ = split
	}
	for e.range_ < 128 {
		e.range_ <<= 1
		if e.bottom&(1<<31) != 0 {
			for i := len(e.output) - 1; i >= 0; i-- {
				e.output[i]++
				if e.output[i] != 0 {
					break
				}
			}
		}
		e.bottom <<= 1
		e.bitCount++
		if e.bitCount == 8 {
			e.output = append(e.output, byte(e.bottom>>24))
			e.bottom &= (1 << 24) - 1
			e.bitCount = 0
		}
	}
}

func (e *boolEncoder) writeLiteral(value int, bits int) {
	for i := bits - 1; i >= 0; i-- {
		e.writeBool(((value>>uint(i))&1) == 1, 128)
	}
}

func (e *boolEncoder) flush() []byte {
	for i := 0; i < 32; i++ {
		e.writeBool(false, 128)
	}
	return e.output
}

// GenerateBlackKeyframe produces a valid VP8 keyframe of the given resolution
// where every macroblock uses DC prediction with all-zero coefficients (black).
func GenerateBlackKeyframe(width, height int) []byte {
	mbW := (width + 15) / 16
	mbH := (height + 15) / 16

	enc := newBoolEncoder()

	// ── Frame header (keyframe-specific) ──
	enc.writeBool(false, 128) // color_space  = YUV
	enc.writeBool(false, 128) // clamping     = 0
	enc.writeBool(false, 128) // segmentation = off
	enc.writeBool(false, 128) // filter_type  = normal
	enc.writeLiteral(0, 6)    // loop_filter_level
	enc.writeLiteral(0, 3)    // sharpness_level
	enc.writeBool(false, 128) // mode_ref_lf_delta = off
	enc.writeLiteral(0, 2)    // log2(dct partitions) = 0 → 1 partition

	// ── Quantization ──
	enc.writeLiteral(10, 7)   // base_qindex
	enc.writeBool(false, 128) // no y1dc delta
	enc.writeBool(false, 128) // no y2dc delta
	enc.writeBool(false, 128) // no y2ac delta
	enc.writeBool(false, 128) // no uvdc delta
	enc.writeBool(false, 128) // no uvac delta

	enc.writeBool(false, 128) // refresh_entropy_probs = 0

	// ── Skip mode ──
	enc.writeBool(true, 128) // mb_no_skip_coeff = 1
	enc.writeLiteral(1, 8)   // prob_skip_false = 1 (nearly all MBs skip)

	// ── Macroblock modes ──
	for row := 0; row < mbH; row++ {
		for col := 0; col < mbW; col++ {
			enc.writeBool(true, 1) // mb_skip = 1

			// Y mode: DC_PRED — kf_ymode tree {145,156,163,128}
			enc.writeBool(true, 145)  // not B_PRED
			enc.writeBool(false, 156) // left branch
			enc.writeBool(false, 163) // DC_PRED

			// UV mode: DC_PRED — kf_uv tree {142,114,183}
			enc.writeBool(false, 142) // DC_PRED
		}
	}

	firstPart := enc.flush()
	firstPartSize := len(firstPart)

	// ── Assemble raw frame ──
	tag := uint32(0) | (1 << 4) | (uint32(firstPartSize) << 5)

	frame := make([]byte, 0, 10+len(firstPart))
	frame = append(frame, byte(tag), byte(tag>>8), byte(tag>>16))
	frame = append(frame, 0x9D, 0x01, 0x2A)

	wb := make([]byte, 2)
	binary.LittleEndian.PutUint16(wb, uint16(width))
	frame = append(frame, wb...)
	binary.LittleEndian.PutUint16(wb, uint16(height))
	frame = append(frame, wb...)

	frame = append(frame, firstPart...)
	return frame
}
