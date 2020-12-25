package semver

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/amane3/goreleaser/internal/pipe"
	"github.com/amane3/goreleaser/pkg/context"
	"github.com/apex/log"
)

// Pipe is a global hook pipe.
type Pipe struct{}

// String is the name of this pipe.
func (Pipe) String() string {
	return "parsing tag"
}

// Run executes the hooks.
func (Pipe) Run(ctx *context.Context) error {
	sv, err := semver.NewVersion(ctx.Git.CurrentTag)
	if err != nil {
		if ctx.Snapshot {
			return pipe.ErrSnapshotEnabled
		}
		if ctx.SkipValidate {
			log.WithError(err).
				WithField("tag", ctx.Git.CurrentTag).
				Warn("current tag is not a semantic tag")
			return pipe.ErrSkipValidateEnabled
		}
		return fmt.Errorf("failed to parse tag %s as semver: %w", ctx.Git.CurrentTag, err)
	}
	ctx.Semver = context.Semver{
		Major:      sv.Major(),
		Minor:      sv.Minor(),
		Patch:      sv.Patch(),
		Prerelease: sv.Prerelease(),
	}
	return nil
}
