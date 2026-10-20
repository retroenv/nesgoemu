package mcpserver

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/retroenv/retrogolib/mcp"
)

// referenceDoc is the tool reference that the docs resource returns.
//
//go:embed reference.md
var referenceDoc string

// docsURI is the static resource URI of the tool reference.
const docsURI = "nesgoemu://docs/mcp"

// screenURITemplate is the resource template of a machine screen.
const screenURITemplate = "nesgoemu://machines/{machine_id}/screen"

// registerResources registers the static and templated resources.
func (s *Server) registerResources() error {
	err := s.mcp.AddResource(mcp.Resource{
		URI:         docsURI,
		Name:        "MCP tool reference",
		Description: "Tool, argument, and resource reference for the nesgoemu MCP server.",
		MimeType:    "text/markdown",
	}, s.readDocs)
	if err != nil {
		return fmt.Errorf("registering docs resource: %w", err)
	}

	err = s.mcp.AddResourceTemplate(mcp.ResourceTemplate{
		URITemplate: screenURITemplate,
		Name:        "Machine screen",
		Description: "Current frame of a machine as a PNG image.",
		MimeType:    "image/png",
	}, s.readScreen)
	if err != nil {
		return fmt.Errorf("registering screen resource: %w", err)
	}
	return nil
}

// readDocs returns the tool reference.
func (s *Server) readDocs(_ context.Context, uri string) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{{
		URI:      uri,
		MimeType: "text/markdown",
		Text:     referenceDoc,
	}}, nil
}

// readScreen returns the current frame of a machine as a PNG.
func (s *Server) readScreen(_ context.Context, uri string) ([]mcp.ResourceContents, error) {
	id, err := screenMachineID(uri)
	if err != nil {
		return nil, err
	}
	machine, err := s.registry.get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", mcp.ErrResourceNotFound, err)
	}
	image, err := machine.screenshot(1)
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{{
		URI:      uri,
		MimeType: "image/png",
		Blob:     image,
	}}, nil
}

// screenMachineID extracts the machine identifier from a screen URI.
func screenMachineID(uri string) (string, error) {
	trimmed := strings.TrimPrefix(uri, "nesgoemu://machines/")
	trimmed = strings.TrimSuffix(trimmed, "/screen")
	if trimmed == "" || trimmed == uri {
		return "", fmt.Errorf("invalid screen URI %q", uri)
	}
	return trimmed, nil
}
