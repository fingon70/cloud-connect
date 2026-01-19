package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fingon70/cloud-connect/internal/model"
)

func TestSyncFileConflictOnLocalAndRemoteChange(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(dest, []byte("local"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	localMtime := time.Unix(2000, 0)
	if err := os.Chtimes(dest, localMtime, localMtime); err != nil {
		t.Fatalf("set mtime: %v", err)
	}

	entry := model.Entry{
		Path:      "/remote/file.txt",
		Size:      5,
		UpdatedAt: time.Unix(3000, 0),
		Chash:     "remote-new",
		Mhash:     "meta-new",
	}
	state := &SyncState{
		Version: 1,
		Files: map[string]FileState{
			entry.Path: {
				RemoteChash: "remote-old",
				RemoteMhash: "meta-old",
				RemoteMtime: 1000,
				RemoteSize:  5,
				LocalMtime:  1000,
				LocalSize:   5,
			},
		},
	}

	syncer := Syncer{}
	outcome, err := syncer.syncFile(contextBackground(), entry, dest, Options{DryRun: true}, state)
	if err != nil {
		t.Fatalf("syncFile error: %v", err)
	}
	if outcome != outcomeConflict {
		t.Fatalf("expected conflict, got %s", outcome)
	}
}

func TestSyncFileDownloadsOnRemoteChange(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(dest, []byte("local"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	localMtime := time.Unix(2000, 0)
	if err := os.Chtimes(dest, localMtime, localMtime); err != nil {
		t.Fatalf("set mtime: %v", err)
	}

	entry := model.Entry{
		Path:      "/remote/file.txt",
		Size:      10,
		UpdatedAt: time.Unix(3000, 0),
		Chash:     "remote-new",
		Mhash:     "meta-new",
	}
	state := &SyncState{
		Version: 1,
		Files: map[string]FileState{
			entry.Path: {
				RemoteChash: "remote-old",
				RemoteMhash: "meta-old",
				RemoteMtime: 2000,
				RemoteSize:  10,
				LocalMtime:  localMtime.Unix(),
				LocalSize:   5,
			},
		},
	}

	syncer := Syncer{}
	outcome, err := syncer.syncFile(contextBackground(), entry, dest, Options{DryRun: true}, state)
	if err != nil {
		t.Fatalf("syncFile error: %v", err)
	}
	if outcome != outcomeDownloaded {
		t.Fatalf("expected downloaded, got %s", outcome)
	}
	updated := state.Files[entry.Path]
	if updated.RemoteChash != entry.Chash || updated.RemoteMhash != entry.Mhash {
		t.Fatalf("state not updated with remote hashes")
	}
	if updated.LocalMtime != entry.UpdatedAt.Unix() || updated.LocalSize != entry.Size {
		t.Fatalf("state not updated with local file info")
	}
}

func TestWriteReport(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "report.json")
	report := syncReport{
		Downloaded:    2,
		Skipped:       1,
		Conflicts:     3,
		ConflictPaths: []string{"/a", "/b"},
	}
	if err := writeReport(path, report); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var decoded syncReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if decoded.Conflicts != report.Conflicts || len(decoded.ConflictPaths) != 2 {
		t.Fatalf("unexpected report content")
	}
}

func TestInferLocalRootRelativeFromNamedRoot(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "hidrive-sync")
	nested := filepath.Join(root, "Foo", "NewDir", "bar.txt")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(nested, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	rel, ok := inferLocalRootRelative(nested)
	if !ok {
		t.Fatalf("expected root relative path")
	}
	if rel != "Foo/NewDir/bar.txt" {
		t.Fatalf("unexpected relative path: %s", rel)
	}
}

func contextBackground() context.Context {
	return context.Background()
}
