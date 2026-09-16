package mapperdb

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/rainbow"
)

// NewRainbow returns a new Rainbow mapper instance.
func NewRainbow(base Base) (bus.Mapper, error) {
	m, err := rainbow.New(base.(*mapperbase.Base))
	if err != nil {
		return nil, fmt.Errorf("creating rainbow mapper: %w", err)
	}
	return m, nil
}
