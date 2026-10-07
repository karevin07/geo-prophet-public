// Package backupdir reads the database dumps the backup service writes and the space left
// on their filesystem. It only reads: dumps are created and pruned by the backup service.
package backupdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type Dir struct {
	Path string
}

// Backups lists completed dumps, newest first. Dumps still being written have a
// .partial name and are left out. A missing directory means no backups yet.
func (dir Dir) Backups() ([]domain.BackupFile, error) {
	entries, err := os.ReadDir(dir.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return []domain.BackupFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read backup directory: %w", err)
	}
	backups := []domain.BackupFile{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "geoprophet-") || filepath.Ext(name) != ".dump" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue // removed while listing
		}
		backups = append(backups, domain.BackupFile{Name: name, Bytes: info.Size(), ModifiedAt: info.ModTime().UTC()})
	}
	slices.SortFunc(backups, func(a, b domain.BackupFile) int { return b.ModifiedAt.Compare(a.ModifiedAt) })
	return backups, nil
}

// Disk reports the filesystem holding the directory.
func (dir Dir) Disk() (domain.DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir.Path, &stat); err != nil {
		return domain.DiskUsage{}, fmt.Errorf("stat backup filesystem: %w", err)
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	// Bavail is what unprivileged writers such as pg_dump can still use.
	free := stat.Bavail * blockSize
	return domain.DiskUsage{TotalBytes: total, UsedBytes: total - stat.Bfree*blockSize, FreeBytes: free}, nil
}
