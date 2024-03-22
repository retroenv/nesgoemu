package debugger

import (
	"encoding/json"
	"fmt"
	"strings"
)

// hexArray implements a byte array alias that JSON marshals to a hex array.
type hexArray []byte

func (h hexArray) MarshalJSON() ([]byte, error) {
	parts := make([]string, len(h))
	for i, b := range h {
		parts[i] = fmt.Sprintf("%02X", b)
	}

	b, err := json.Marshal(parts)
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON: %w", err)
	}
	return b, nil
}

// hexArrayCombined implements a byte array alias that JSON marshals to a hex string.
type hexArrayCombined []byte

func (h hexArrayCombined) MarshalJSON() ([]byte, error) {
	buf := strings.Builder{}

	for _, b := range h {
		s := fmt.Sprintf("%02X", b)
		buf.WriteString(s)
	}

	b, err := json.Marshal(buf.String())
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON: %w", err)
	}
	return b, nil
}

// hexByte implements byte alias that JSON marshals to a hex string.
type hexByte uint8

func (h hexByte) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%02X", h)
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON: %w", err)
	}
	return b, nil
}

// hexWord implements word alias that JSON marshals to a hex string.
type hexWord uint16

func (h hexWord) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%04X", h)
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON: %w", err)
	}
	return b, nil
}

// hexDword implements qword alias that JSON marshals to a hex string.
type hexQword uint64

func (h hexQword) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%08X", h)
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON: %w", err)
	}
	return b, nil
}

// nolint: unparam
func bytesToSliceArrayCombined(data []byte, rows, width int) []hexArrayCombined {
	result := make([]hexArrayCombined, 0, rows)

	for row := range rows {
		offset := row * width
		result = append(result, data[offset:offset+width])
	}

	return result
}
