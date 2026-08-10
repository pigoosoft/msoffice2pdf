package converter

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Engine is a loadable conversion backend.
type Engine interface {
	Name() string
	Validate() error
	Convert(ctx context.Context, srcPath, dstPath string) error
	ProcessImages() []string
}

var (
	_ Converter = (*routingConverter)(nil)
	_ Engine    = (*openOfficeEngine)(nil)
	_ Engine    = (*comBackendEngine)(nil)
)

type routingConverter struct {
	engines    map[string]Engine
	extEngines map[string]string
}

func (r *routingConverter) Convert(ctx context.Context, srcPath, dstPath string) error {
	bare := strings.ToLower(strings.TrimPrefix(filepath.Ext(srcPath), "."))
	name, ok := r.extEngines[bare]
	if !ok || name == "" {
		return fmt.Errorf("converter: no engine mapped for .%s", bare)
	}
	eng, ok := r.engines[name]
	if !ok {
		return fmt.Errorf("converter: engine %q not loaded", name)
	}
	return eng.Convert(ctx, srcPath, dstPath)
}

// comBackendEngine adapts an existing COM/subprocess Converter to the Engine interface.
type comBackendEngine struct{ inner Converter }

func (c *comBackendEngine) Name() string { return "com" }

func (c *comBackendEngine) Validate() error { return nil }

func (c *comBackendEngine) ProcessImages() []string { return nil }

func (c *comBackendEngine) Convert(ctx context.Context, src, dst string) error {
	return c.inner.Convert(ctx, src, dst)
}
