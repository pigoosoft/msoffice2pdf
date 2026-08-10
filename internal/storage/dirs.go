package storage

import (
	"fmt"
	"os"

	"msoffice2pdf/internal/config"
)

func EnsureDirs(cfg config.StorageConfig) error {
	dirs := []string{cfg.UploadDir, cfg.OutputDir, cfg.TrashDir, cfg.ExpiredDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}
