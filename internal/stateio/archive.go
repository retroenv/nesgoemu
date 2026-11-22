// Package stateio transfers stopped device state without pointers or callbacks.
package stateio

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
)

// Device transfers the mutable fields that affect emulation.
type Device interface {
	TransferState(*Archive) error
}

// Archive reads or writes a versioned system record. Its caller owns the format.
// Decode into a new system and publish it only after all records are valid.
type Archive struct {
	decoder *gob.Decoder
	encoder *gob.Encoder
}

// NewWriter creates a state writer.
func NewWriter(w io.Writer) *Archive {
	return &Archive{
		encoder: gob.NewEncoder(w),
	}
}

// NewReader creates a state reader. The caller must limit the input size.
func NewReader(r io.Reader) *Archive {
	return &Archive{
		decoder: gob.NewDecoder(r),
	}
}

// Reading reports whether the archive restores a record.
func (a *Archive) Reading() bool { return a.decoder != nil }

// End checks that a reader has no extra records.
func (a *Archive) End() error {
	var extra bool
	err := a.decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading state end: %w", err)
	}
	return errors.New("state has an extra record")
}

// Values transfers scalar values, arrays, and records with exported fields.
func (a *Archive) Values(values ...any) error {
	for i, value := range values {
		var err error
		if a.decoder != nil {
			err = a.decoder.Decode(value)
		} else {
			err = a.encoder.Encode(value)
		}

		if err != nil {
			return fmt.Errorf("state field %d: %w", i, err)
		}
	}

	return nil
}

// Bytes transfers a fixed-size memory buffer and keeps its backing array.
func (a *Archive) Bytes(data []byte) error {
	if !a.Reading() {
		return a.Values(data)
	}

	var next []byte
	if err := a.Values(&next); err != nil {
		return err
	}
	if len(next) != len(data) {
		return fmt.Errorf("state memory size %d differs from %d", len(next), len(data))
	}
	copy(data, next)

	return nil
}

// Devices transfers component state in the declared order.
func (a *Archive) Devices(devices ...Device) error {
	for i, device := range devices {
		if err := device.TransferState(a); err != nil {
			return fmt.Errorf("state component %d: %w", i, err)
		}
	}

	return nil
}
