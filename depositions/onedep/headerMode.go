package onedep

import (
	"encoding/binary"
	"math"
)

type HeaderMode func(data []byte) interface{}

var typeMap = map[uint32]HeaderMode{
	0: func(data []byte) interface{} {
		return int8(data[0]) // 8-bit signed integer
	},
	1: func(data []byte) interface{} {
		return int16(binary.LittleEndian.Uint16(data)) // 16-bit signed integer
	},
	2: func(data []byte) interface{} {
		return math.Float32frombits(binary.LittleEndian.Uint32(data)) // 32-bit signed real
	},
	3: func(data []byte) interface{} {
		// Complex 16-bit integers (for simplicity, return raw data)
		return []int16{int16(binary.LittleEndian.Uint16(data[:2])), int16(binary.LittleEndian.Uint16(data[2:]))}
	},
	4: func(data []byte) interface{} {
		// Complex 32-bit reals
		return []float32{math.Float32frombits(binary.LittleEndian.Uint32(data[:4])), math.Float32frombits(binary.LittleEndian.Uint32(data[4:]))}
	},
	6: func(data []byte) interface{} {
		return binary.LittleEndian.Uint16(data) // 16-bit unsigned integer
	},
	12: func(data []byte) interface{} {
		return math.Float32frombits(binary.LittleEndian.Uint32(data)) // 16-bit float (IEEE754)
	},
	101: func(data []byte) interface{} {
		// 4-bit data packed two per byte (handle accordingly, for now just returning raw bytes)
		return data
	},
}
