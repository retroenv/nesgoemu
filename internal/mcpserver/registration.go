package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/retroenv/retrogolib/mcp"
)

const (
	waitingForMachineFormat = "waiting for machine: %w"
	invalidLimitFormat      = "limit %d is outside 1 to %d"
	negativeOffsetFormat    = "offset %d is negative"
	controllerNumberFormat  = "controller number %d is not 1 or 2"
	openingFileFormat       = "opening file: %w"
	readingFileFormat       = "reading file: %w"

	schemaEnum             = "enum"
	schemaItems            = "items"
	schemaMaxItems         = "maxItems"
	schemaMaximum          = "maximum"
	schemaMinItems         = "minItems"
	schemaMinimum          = "minimum"
	schemaOneOf            = "oneOf"
	schemaProperties       = "properties"
	schemaRequired         = "required"
	schemaType             = "type"
	schemaUniqueItems      = "uniqueItems"
	invalidArgumentsFormat = "Invalid arguments: %v"

	fieldAction       = "action"
	fieldBytes        = "bytes"
	fieldCapture      = "capture"
	fieldFrames       = "frames"
	fieldInputs       = "inputs"
	fieldInstructions = "instructions"
	fieldLimit        = "limit"
	fieldMemory       = "memory"
	fieldMode         = "mode"
	fieldOffset       = "offset"
	fieldOp           = "op"
	fieldScale        = "scale"
	fieldScreenshot   = "screenshot"
	fieldSearchID     = "search_id"
	fieldSections     = "sections"
	fieldValue        = "value"
)

// Tool names are shared by registration, dispatch, and callers.
const (
	addBreakpointToolName    = "add_breakpoint"
	callStackToolName        = "call_stack"
	cdlExportToolName        = "cdl_export"
	cdlReadToolName          = "cdl_read"
	cdlResetToolName         = "cdl_reset"
	cdlSetEnabledToolName    = "cdl_set_enabled"
	checkpointToolName       = "checkpoint"
	coverageToolName         = "coverage"
	createMachineToolName    = "create_machine"
	destroyMachineToolName   = "destroy_machine"
	disassembleToolName      = "disassemble"
	eventLogToolName         = "event_log"
	findAccessorsToolName    = "find_accessors"
	forkMachineToolName      = "fork_machine"
	getTraceToolName         = "get_trace"
	inspectMachineToolName   = "inspect_machine"
	listBreakpointsToolName  = "list_breakpoints"
	listMachinesToolName     = "list_machines"
	memorySearchToolName     = "memory_search"
	nametablesToolName       = "nametables"
	patternTablesToolName    = "pattern_tables"
	ramDiffToolName          = "ram_diff"
	readMemoryToolName       = "read_memory"
	recordAudioToolName      = "record_audio"
	removeBreakpointToolName = "remove_breakpoint"
	resetToolName            = "reset"
	restoreToolName          = "restore"
	restoreMachineToolName   = "restore_machine"
	runFramesToolName        = "run_frames"
	runUntilToolName         = "run_until"
	screenshotToolName       = "screenshot"
	setButtonsToolName       = "set_buttons"
	stepToolName             = "step"
	traceStartToolName       = "trace_start"
	traceStopToolName        = "trace_stop"
	watchWritesToolName      = "watch_writes"
	writeMemoryToolName      = "write_memory"
)

// beginOperation includes queue time in the operation deadline.
// A machine request keeps its gate until the complete response is built.
func (s *Server) beginOperation(ctx context.Context, name string,
	req mcp.ToolRequest) (context.Context, func(), error) {

	var arguments struct {
		MachineID string `json:"machine_id"`
		Timeout   string `json:"timeout"`
	}
	if len(req.Arguments) > 0 {
		if err := json.Unmarshal(req.Arguments, &arguments); err != nil {
			return nil, nil, fmt.Errorf("reading operation arguments: %w", err)
		}
	}

	timeout, err := parseTimeout(arguments.Timeout)
	if err != nil {
		return nil, nil, err
	}
	if name == recordAudioToolName && arguments.Timeout == "" {
		timeout = maxRunTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	if arguments.MachineID == "" {
		if err := ctx.Err(); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("starting operation: %w", err)
		}
		return ctx, cancel, nil
	}
	machine, err := s.registry.get(arguments.MachineID)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	release, err := machine.acquireRequest(ctx)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return ctx, func() {
		release()
		cancel()
	}, nil
}

// acquireRequest serializes complete tool calls and permits cancellation in the queue.
func (m *Machine) acquireRequest(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(waitingForMachineFormat, err)
	}

	select {
	case m.requests <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-m.requests
			return nil, fmt.Errorf(waitingForMachineFormat, err)
		}
		return func() {
			m.touch()
			<-m.requests
		}, nil

	case <-ctx.Done():
		return nil, fmt.Errorf(waitingForMachineFormat, ctx.Err())
	}
}

// operationMiddleware keeps the machine gate through result creation.
func (s *Server) operationMiddleware(name string, next mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
		ctx, finish, err := s.beginOperation(ctx, name, req)
		if err != nil {
			return mcp.Errorf("%v", err), nil
		}
		defer finish()

		return next(ctx, req) //nolint:wrapcheck // Preserve protocol errors.
	}
}

// addTool selects the server's result contract and operation lifetime.
func addTool[In, Out any](s *Server, tool mcp.Tool, handler func(context.Context, mcp.ToolRequest, In) (Out, error),
	content ...func(Out) []mcp.Content) error {

	options := mcp.TypedToolOptions[Out]{
		GenerateOutputSchema: true,

		Middleware: s.operationMiddleware,
	}
	if len(content) > 0 {
		options.Content = content[0]
	}

	return registrationError(mcp.AddTool(s.mcp, tool, handler, options), tool.Name)
}

// addImageTool checks raw image-tool arguments against the advertised schema.
func addImageTool[In any](s *Server, tool mcp.Tool, handler mcp.ToolHandler) error {
	schema, err := mcp.SchemaFor(reflect.TypeFor[In]())
	if err != nil {
		return registrationError(fmt.Errorf("generating image arguments: %w", err), tool.Name)
	}
	validator, err := mcp.CompileSchema(schema)
	if err != nil {
		return registrationError(fmt.Errorf("compiling image arguments: %w", err), tool.Name)
	}
	tool.InputSchema = schema
	wrapped := s.operationMiddleware(tool.Name, handler)

	err = s.mcp.AddTool(tool, func(ctx context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
		arguments := req.Arguments
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}

		if err := validator.Validate(arguments); err != nil {
			return mcp.Errorf(invalidArgumentsFormat, err), nil
		}

		return wrapped(ctx, req) //nolint:wrapcheck // Preserve protocol errors.
	})

	return registrationError(err, tool.Name)
}

// inputSchema adds operation constraints to a generated object schema.
func inputSchema(kind reflect.Type, configure func(map[string]any)) (json.RawMessage, error) {
	raw, err := mcp.SchemaFor(kind)
	if err != nil {
		return nil, fmt.Errorf("generating argument schema: %w", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("reading argument schema: %w", err)
	}
	configure(schema)
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("encoding argument schema: %w", err)
	}
	return data, nil
}

// schemaProperty accesses a field in a schema generated from a known Go struct.
func schemaProperty(schema map[string]any, field string) map[string]any {
	return schema[schemaProperties].(map[string]any)[field].(map[string]any)
}

// registrationError wraps a tool or resource registration error.
func registrationError(err error, name string) error {
	if err != nil {
		return fmt.Errorf("registering %s: %w", name, err)
	}
	return nil
}
