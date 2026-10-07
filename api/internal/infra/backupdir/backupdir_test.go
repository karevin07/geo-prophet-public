package backupdir_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/infra/backupdir"
)

func TestBackupsListsCompletedDumpsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, age time.Duration) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("dump"), 0o644); err != nil {
			t.Fatal(err)
		}
		modified := time.Now().Add(-age)
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	write("geoprophet-20260929T000000Z.dump", 48*time.Hour)
	write("geoprophet-20260930T000000Z.dump", 24*time.Hour)
	write(".geoprophet-20261001T000000Z.dump.partial", 0)
	write("notes.txt", 0)

	backups, err := backupdir.Dir{Path: dir}.Backups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 || backups[0].Name != "geoprophet-20260930T000000Z.dump" || backups[0].Bytes != 4 {
		t.Fatalf("backups = %+v", backups)
	}
	if missing, err := (backupdir.Dir{Path: filepath.Join(dir, "missing")}).Backups(); err != nil || len(missing) != 0 {
		t.Fatalf("missing directory = %v, %v", missing, err)
	}
	disk, err := backupdir.Dir{Path: dir}.Disk()
	if err != nil || disk.TotalBytes == 0 || disk.UsedBytes > disk.TotalBytes {
		t.Fatalf("disk = %+v, %v", disk, err)
	}
}
