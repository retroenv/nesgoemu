package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"time"

	neslib "github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/mcp"
)

const maxMemorySearches = 16

func (s *Server) memorySearch(ctx context.Context, _ mcp.ToolRequest, in memorySearchArgs) (memorySearchResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return memorySearchResult{}, err
	}
	if in.Action == searchStart {
		if in.SearchID != "" {
			return memorySearchResult{}, errors.New("action start does not accept search_id")
		}
		return machine.startSearch()
	}
	if in.SearchID == "" {
		return memorySearchResult{}, errors.New("search_id is required; call memory_search with action start first")
	}

	switch in.Action {
	case searchFilter:
		return machine.prepareSearchFilter(ctx, in)

	case searchList:
		limit, offset, err := pageArguments(in.Limit, in.Offset, defaultSearchPage)
		if err != nil {
			return memorySearchResult{}, err
		}
		return machine.searchPage(in.SearchID, limit, offset)

	case searchRelease:
		return machine.releaseSearch(in.SearchID)

	default:
		return memorySearchResult{}, fmt.Errorf("unknown search action %q", in.Action)
	}
}

func (m *Machine) startSearch() (memorySearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.searches) >= maxMemorySearches {
		return memorySearchResult{}, fmt.Errorf("search limit %d reached; release a search first", maxMemorySearches)
	}
	id, err := newHandleID()
	if err != nil {
		return memorySearchResult{}, err
	}
	candidates := make([]memorySearchEntry, 0, neslib.RAMEndAddress+1)
	for address := range neslib.RAMEndAddress + 1 {
		value, ok := m.sys.InspectRAM(uint16(address))
		if ok {
			candidates = append(candidates, memorySearchEntry{
				Address: uint16(address),
				Value:   value,
			})
		}
	}

	if m.searches == nil {
		m.searches = make(map[string]*memorySearch)
	}

	m.searches[id] = &memorySearch{
		candidates: candidates,
	}

	return memorySearchResult{
		SearchID:   id,
		Candidates: len(candidates),
	}, nil
}

func (m *Machine) searchPage(id string, limit, offset int) (memorySearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	search, err := m.searchLocked(id)
	if err != nil {
		return memorySearchResult{}, err
	}
	result := memorySearchResult{
		SearchID:   id,
		Candidates: len(search.candidates),
	}
	if offset < len(search.candidates) {
		end := offset + min(limit, len(search.candidates)-offset)
		result.Results = slices.Clone(search.candidates[offset:end])
	}

	return result, nil
}

func (m *Machine) releaseSearch(id string) (memorySearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := m.searchLocked(id); err != nil {
		return memorySearchResult{}, err
	}
	delete(m.searches, id)

	return memorySearchResult{
		SearchID: id,
		Released: true,
	}, nil
}

func (m *Machine) searchLocked(id string) (*memorySearch, error) {
	search, ok := m.searches[id]
	if !ok {
		return nil, fmt.Errorf("unknown search_id %q; start a new search", id)
	}
	return search, nil
}

func (m *Machine) prepareSearchFilter(ctx context.Context, in memorySearchArgs) (memorySearchResult, error) {
	if !validSearchOperation(in.Op) {
		return memorySearchResult{}, fmt.Errorf("unknown search filter %q", in.Op)
	}
	if in.Op == searchValue && in.Value == nil {
		return memorySearchResult{}, errors.New("the value filter requires value")
	}
	value := valueOr(in.Value, 0)
	if value < 0 || value > math.MaxUint8 {
		return memorySearchResult{}, errors.New("value must be from 0 to 255")
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return memorySearchResult{}, err
	}
	inputs, err := parseInputs(in.Inputs)
	if err != nil {
		return memorySearchResult{}, err
	}

	for _, input := range inputs {
		if input.Frame >= in.Frames {
			return memorySearchResult{}, errors.New("input frame is outside the filter run")
		}
	}

	return m.filterSearch(ctx, in, inputs, timeout)
}

// filterSearch checks the handle before it runs time or changes the baseline.
func (m *Machine) filterSearch(ctx context.Context, in memorySearchArgs, inputs []controllerInput,
	timeout time.Duration) (memorySearchResult, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	search, err := m.searchLocked(in.SearchID)
	if err != nil {
		return memorySearchResult{}, err
	}
	var run *runSummary
	if in.Frames > 0 {
		result, err := m.runFramesLocked(ctx, in.Frames, inputs, timeout)
		if err != nil {
			return memorySearchResult{}, err
		}
		summary := summarizeRun(result)
		run = &summary
	}

	matches := make([]memorySearchEntry, 0, len(search.candidates))
	for _, entry := range search.candidates {
		current, ok := m.sys.InspectRAM(entry.Address)
		if ok && searchMatches(in.Op, entry.Value, current, byte(valueOr(in.Value, 0))) {
			matches = append(matches, memorySearchEntry{
				Address: entry.Address,
				Value:   current,
			})
		}
	}

	search.candidates = matches

	return memorySearchResult{
		SearchID:   in.SearchID,
		Candidates: len(matches),
		Run:        run,
	}, nil
}

type searchAction string

const (
	searchStart   searchAction = "start"
	searchFilter  searchAction = "filter"
	searchList    searchAction = "list"
	searchRelease searchAction = "release"
)

type memorySearch struct {
	candidates []memorySearchEntry
}

type memorySearchArgs struct {
	Action    searchAction `json:"action" description:"start creates a search. filter, list, and release require search_id."`
	MachineID string       `json:"machine_id" description:"Identifier returned by create_machine."`
	SearchID  string       `json:"search_id,omitempty" description:"Handle returned by action start. Required for filter, list, and release."`

	Op    searchOperation `json:"op,omitempty" description:"Filter: equal, changed, unchanged, increased, decreased, value."`
	Value *int            `json:"value,omitempty" minimum:"0" maximum:"255" description:"Required only for the value filter."`

	Frames  uint64     `json:"frames,omitempty" description:"Frames to run before filtering."`
	Inputs  []inputArg `json:"inputs,omitempty" description:"Controller changes before filtering."`
	Timeout string     `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`

	Limit  *int `json:"limit,omitempty" description:"Page size for list. Default 50; maximum 10000."`
	Offset *int `json:"offset,omitempty" description:"Start index for list. Default 0."`
}

type memorySearchResult struct {
	SearchID string `json:"search_id"`

	Candidates int                 `json:"candidates"`
	Released   bool                `json:"released,omitempty"`
	Results    []memorySearchEntry `json:"results,omitempty"`
	Run        *runSummary         `json:"run,omitempty"`
}

func memorySearchInputSchema() ([]byte, error) {
	return inputSchema(reflect.TypeFor[memorySearchArgs](), configureSearchSchema)
}

func configureSearchSchema(schema map[string]any) {
	schemaProperty(schema, fieldAction)[schemaEnum] = []searchAction{searchStart, searchFilter, searchList, searchRelease}
	schemaProperty(schema, fieldOp)[schemaEnum] = []searchOperation{
		searchEqual, searchChanged, searchUnchanged, searchIncreased, searchDecreased, searchValue,
	}
	schema[schemaOneOf] = []any{
		searchActionSchema(searchStart, fieldSearchID, fieldOp, fieldValue, fieldFrames, fieldInputs, fieldLimit, fieldOffset),
		searchActionSchema(searchFilter, fieldLimit, fieldOffset),
		searchActionSchema(searchList, fieldOp, fieldValue, fieldFrames, fieldInputs),
		searchActionSchema(searchRelease, fieldOp, fieldValue, fieldFrames, fieldInputs, fieldLimit, fieldOffset),
	}
}

func searchActionSchema(action searchAction, excluded ...string) map[string]any {
	properties := map[string]any{
		fieldAction: map[string]any{schemaEnum: []searchAction{action}},
	}

	for _, field := range excluded {
		properties[field] = false
	}

	branch := map[string]any{
		schemaProperties: properties,
	}
	if action != searchStart {
		branch[schemaRequired] = []string{fieldSearchID}
	}

	if action == searchFilter {
		branch[schemaRequired] = []string{fieldSearchID, fieldOp}
		branch[schemaOneOf] = []any{
			map[string]any{
				schemaProperties: map[string]any{fieldOp: map[string]any{schemaEnum: []searchOperation{searchValue}}},
				schemaRequired:   []string{fieldValue},
			},
			map[string]any{
				schemaProperties: map[string]any{
					fieldOp: map[string]any{
						schemaEnum: []searchOperation{searchEqual, searchChanged, searchUnchanged, searchIncreased, searchDecreased},
					},
					fieldValue: false,
				},
			},
		}
	}

	return branch
}
