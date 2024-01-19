package debugger

import (
	"encoding/json"
	"net/http"
)

func (d *Debugger) mapperState(w http.ResponseWriter, _ *http.Request) {
	state := d.bus.Mapper.State()

	_ = json.NewEncoder(w).Encode(state)
}
