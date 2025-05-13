package vgm

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/elemir/nukeykt"
)

type VGM struct {
	VGMHeader

	data   io.ReadSeeker
	ym3438 *nukeykt.YM3438

	tail int

	inStart              float64
	lastSmplL, lastSmplR float64

	secondRun bool
}

func New(vgmData io.ReadSeeker, sampleRate uint32) (*VGM, error) {
	var vgmHeader VGMHeader

	err := binary.Read(vgmData, binary.LittleEndian, &vgmHeader)
	if err != nil {
		return nil, fmt.Errorf("read VGM header: %w", err)
	}

	if string(vgmHeader.FccVGM[:]) != "Vgm " {
		return nil, fmt.Errorf("unknown file type")
	}

	if vgmHeader.Version < 0x150 {
		return nil, fmt.Errorf("unsupported version: %x", vgmHeader.Version)
	}

	dataOffset := int64(vgmHeader.DataOffset) + 0x34
	_, err = vgmData.Seek(dataOffset, 0)
	if err != nil {
		return nil, fmt.Errorf("seek to data section: %w", err)
	}

	var ym3438 nukeykt.YM3438

	nukeykt.OPN2_Reset(&ym3438, sampleRate, vgmHeader.HzYM2612)
	nukeykt.OPN2_SetChipType(nukeykt.ModeYM2612)

	return &VGM{
		VGMHeader: vgmHeader,
		ym3438:    &ym3438,
		data:      vgmData,
	}, nil
}

func (vgm *VGM) Read(out []byte) (int, error) {
	var smpl int
cycle:
	for {
		var shift int
		var cmd uint8
		err := binary.Read(vgm.data, binary.LittleEndian, &cmd)
		if err != nil {
			return smpl, fmt.Errorf("read next command: %w", err)
		}

		var args1 Args1
		var args2 Args2

		switch cmd {
		// SN76489/SN76496
		case 0x4f, 0x50:
			err = binary.Read(vgm.data, binary.LittleEndian, &args1)
		// YMF271
		case 0xb4:
			err = binary.Read(vgm.data, binary.LittleEndian, &args2)
		// YM2608
		case 0x56, 0x57:
			err = binary.Read(vgm.data, binary.LittleEndian, &args2)
		// YM2612
		case 0x52, 0x53:
			port := cmd & 0x01
			err = binary.Read(vgm.data, binary.LittleEndian, &args2)
			nukeykt.OPN2_WriteBuffered(vgm.ym3438, uint32(port<<1)|0x00, args2[0])
			nukeykt.OPN2_WriteBuffered(vgm.ym3438, uint32(port<<1)|0x01, args2[1])
		case 0x61:
			err = binary.Read(vgm.data, binary.LittleEndian, &args2)
			shift = int(args2[0]) + int(args2[1])<<8
		case 0x62:
			shift = 735
		case 0x63:
			shift = 882
		case 0x66:
			return smpl, io.EOF
		default:
			if cmd&0x70 == 0x70 {
				shift = int(cmd&0xf) + 1
			} else {
				return smpl, fmt.Errorf("unknown command: 0x%x", cmd)
			}
		}

		var buf [2]int32

		shift += vgm.tail
		for i := range shift {
			if smpl >= len(out) {
				if shift-i > 0 {
					vgm.tail = shift - i
				}
				break cycle
			}
			nukeykt.OPN2_GenerateResampled(vgm.ym3438, buf[:])
			out[smpl] = byte(buf[0] & 0xff)
			out[smpl+1] = byte(buf[0] >> 8)
			out[smpl+2] = byte(buf[1] & 0xff)
			out[smpl+3] = byte(buf[1] >> 8)

			smpl += 4
		}

		vgm.tail = 0
	}

	return smpl, nil
}
