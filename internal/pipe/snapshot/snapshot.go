// Package snapshot provides the snapshotting functionality to goreleaser.
package snapshot

import (
	"fmt"

	"github.com/amane3/goreleaser/internal/pipe"
	"github.com/amane3/goreleaser/internal/tmpl"
	"github.com/amane3/goreleaser/pkg/context"
)

// Pipe for checksums.
type Pipe struct{}

func (Pipe) String() string {
	return "snapshotting"
}

// Default sets the pipe defaults.
func (Pipe) Default(ctx *context.Context) error {
	if ctx.Config.Snapshot.NameTemplate == "" {
		ctx.Config.Snapshot.NameTemplate = "{{ .Tag }}-SNAPSHOT-{{ .ShortCommit }}"
	}
	return nil
}

func (Pipe) Run(ctx *context.Context) error {
	if !ctx.Snapshot {
		return pipe.Skip("not a snapshot")
	}
	name, err := tmpl.New(ctx).Apply(ctx.Config.Snapshot.NameTemplate)
	if err != nil {
		return fmt.Errorf("failed to generate snapshot name: %w", err)
	}
	if name == "" {
		return fmt.Errorf("empty snapshot name")
	}
	ctx.Version = name
	return nil
}
