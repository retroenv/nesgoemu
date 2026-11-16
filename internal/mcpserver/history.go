package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const (
	maxHistoryEvents = 65_536
	maxHistoryBytes  = 16 << 20
	maxCheckpoints   = 1024
	maxWriteLength   = 4096

	// The metadata budget covers field names, timestamps, addresses, and eight
	// button names. Each byte value needs at most three digits and a comma.
	eventMetadataBytes = 256
	eventValueBytes    = 4
)

// checkHistoryCapacity checks the complete call before it changes the system.
// The caller holds the machine mutex. Unused input slots are not retained.
func (m *Machine) checkHistoryCapacity(events, values int) error {
	if events > maxHistoryEvents-len(m.events) {
		return errors.New("event history limit reached; use a hard reset or create a new machine")
	}
	bytes := events*eventMetadataBytes + values*eventValueBytes
	if bytes > maxHistoryBytes-m.historyBytes {
		return errors.New("event history byte limit reached; use a hard reset or create a new machine")
	}
	return nil
}

// historyBytes checks imported history before a ROM is loaded or replayed.
func historyBytes(events []machineEvent) (int, error) {
	if len(events) > maxHistoryEvents {
		return 0, errors.New("event history limit exceeded")
	}
	bytes := 0
	for _, event := range events {
		if err := validateHistoryEvent(event); err != nil {
			return 0, err
		}
		bytes += eventMetadataBytes + len(event.Bytes)*eventValueBytes
		if bytes > maxHistoryBytes {
			return 0, errors.New("event history byte limit exceeded")
		}
	}

	return bytes, nil
}

func validateHistoryEvent(event machineEvent) error {
	if len(event.Buttons) > len(buttonNames) || len(event.Bytes) > maxWriteLength {
		return errors.New("event payload exceeds the limit")
	}

	for _, name := range event.Buttons {
		if _, ok := buttonNames[name]; !ok {
			return fmt.Errorf("unknown event button %q", name)
		}
	}

	if _, err := toBytes(event.Bytes); err != nil {
		return err
	}
	if event.Controller != nil && *event.Controller != 1 && *event.Controller != 2 {
		return errors.New("event controller must be 1 or 2")
	}

	switch event.Kind {
	case eventInput, eventReset:
	case eventPoke:
		if event.Address == nil || len(event.Bytes) == 0 {
			return errors.New("poke event requires an address and bytes")
		}

	default:
		return fmt.Errorf("unknown event kind %q", event.Kind)
	}

	return nil
}

// encodeEventLog refuses a file that the event log reader cannot load.
func encodeEventLog(logFile eventLogFile, limit int) ([]byte, error) {
	data, err := json.Marshal(logFile)
	if err != nil {
		return nil, fmt.Errorf("encoding event log: %w", err)
	}
	if len(data) > limit {
		return nil, fmt.Errorf("event log exceeds the file limit of %d bytes", limit)
	}
	return data, nil
}

func configureWriteSchema(schema map[string]any) {
	values := schemaProperty(schema, fieldBytes)
	values[schemaMinItems] = 1
	values[schemaMaxItems] = maxWriteLength
	items := values[schemaItems].(map[string]any)
	items[schemaMinimum] = 0
	items[schemaMaximum] = math.MaxUint8
}
