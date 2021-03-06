package extrafiles

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amane3/goreleaser/pkg/config"
	"github.com/apex/log"
	"github.com/goreleaser/fileglob"
)

// Find resolves extra files globs et al into a map of names/paths or an error.
func Find(files []config.ExtraFile) (map[string]string, error) {
	var result = map[string]string{}
	for _, extra := range files {
		if extra.Glob == "" {
			continue
		}
		files, err := fileglob.Glob(extra.Glob)
		if err != nil {
			return result, fmt.Errorf("globbing failed for pattern %s: %w", extra.Glob, err)
		}
		for _, file := range files {
			info, err := os.Stat(file)
			if err == nil && info.IsDir() {
				log.Debugf("ignoring directory %s", file)
				continue
			}
			var name = filepath.Base(file)
			if old, ok := result[name]; ok {
				log.Warnf("overriding %s with %s for name %s", old, file, name)
			}
			result[name] = file
		}
	}
	return result, nil
}
