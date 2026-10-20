package mcpserver

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Address is a 16-bit address that accepts an integer, "$C000", or "0xC000".
type Address uint16

// UnmarshalJSON decodes an integer, "$C000", or "0xC000".
func (a *Address) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*a = 0
		return nil
	}

	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decoding address: %w", err)
		}
		parsed, err := parseAddressString(value)
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	}

	number, err := strconv.ParseUint(text, 0, 16)
	if err != nil {
		return fmt.Errorf("decoding address %q: %w", text, err)
	}
	*a = Address(number)
	return nil
}

// String returns the address as a "$C000" string.
func (a Address) String() string {
	return fmt.Sprintf("$%04X", uint16(a))
}

// MarshalJSON emits the address as a "$C000" string.
func (a Address) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(a.String())
	if err != nil {
		return nil, fmt.Errorf("encoding address: %w", err)
	}
	return data, nil
}

// JSONSchema makes the MCP schema accept an integer or a hexadecimal string.
func (Address) JSONSchema() json.RawMessage {
	return json.RawMessage(`{"type":["integer","string"]}`)
}

// parseAddressString decodes "$C000", "0xC000", or a bare hexadecimal string.
func parseAddressString(value string) (Address, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "$")
	trimmed = strings.TrimPrefix(trimmed, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")

	number, err := strconv.ParseUint(trimmed, 16, 16)
	if err != nil {
		return 0, fmt.Errorf("decoding address %q: %w", value, err)
	}
	return Address(number), nil
}
