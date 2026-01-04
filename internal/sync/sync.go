package sync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fingon70/cloud-connect/internal/api"
	"github.com/fingon70/cloud-connect/internal/model"
	"github.com/fingon70/cloud-connect/internal/ui"
)

type Options struct {
	DryRun     bool
	ReportPath string
}

type Syncer struct {
	Client *api.Client
}

type syncOutcome string

const (
	outcomeDownloaded syncOutcome = "downloaded"
	outcomeSkipped    syncOutcome = "skipped"
	outcomeConflict   syncOutcome = "conflict"
)

type syncReport struct {
	Downloaded    int      `json:"downloaded"`
	Skipped       int      `json:"skipped"`
	Conflicts     int      `json:"conflicts"`
	ConflictPaths []string `json:"conflicts_list,omitempty"`
}

func (s *Syncer) Sync(ctx context.Context, remotePath, localRoot string, opts Options) error {
	if s.Client == nil {
		return errors.New("missing api client")
	}

	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return errors.New("remote path is required")
	}

	if err := s.ensureDir(localRoot, opts); err != nil {
		return err
	}

	localBase, err := s.localBase(ctx, remotePath, localRoot, opts)
	if err != nil {
		return err
	}

	state, err := LoadState()
	if err != nil {
		return err
	}

	entries, err := s.walk(ctx, remotePath)
	if err != nil {
		return err
	}

	report := syncReport{}

	for _, entry := range entries {
		rel := relPath(remotePath, entry.Path)
		if rel == "" {
			continue
		}
		localPath := filepath.Join(localBase, rel)
		switch entry.Type {
		case model.EntryFolder:
			if err := s.ensureDir(localPath, opts); err != nil {
				return err
			}
		case model.EntryFile:
			outcome, err := s.syncFile(ctx, entry, localPath, opts, state)
			if err != nil {
				return err
			}
			switch outcome {
			case outcomeDownloaded:
				report.Downloaded++
			case outcomeConflict:
				report.Conflicts++
				report.ConflictPaths = append(report.ConflictPaths, entry.Path)
			case outcomeSkipped:
				report.Skipped++
			}
		}
	}

	if err := SaveState(state); err != nil {
		return err
	}

	if opts.ReportPath != "" {
		if err := writeReport(opts.ReportPath, report); err != nil {
			return err
		}
	}

	ui.Infof("sync summary: downloaded=%d skipped=%d conflicts=%d", report.Downloaded, report.Skipped, report.Conflicts)
	return nil
}

func (s *Syncer) localBase(ctx context.Context, remotePath, localRoot string, opts Options) (string, error) {
	localBase := localRoot
	if remotePath == "/" {
		return localBase, nil
	}

	trimmed := strings.TrimPrefix(remotePath, "/")
	if strings.HasPrefix(remotePath, "/users/") && s.Client != nil {
		user, err := s.Client.GetUser(ctx)
		if err == nil && user.Alias != "" {
			prefix := "/users/" + user.Alias
			if remotePath == prefix {
				return localBase, nil
			}
			if strings.HasPrefix(remotePath, prefix+"/") {
				trimmed = strings.TrimPrefix(remotePath, prefix+"/")
			}
		}
	}

	localBase = filepath.Join(localRoot, trimmed)
	if err := s.ensureDir(localBase, opts); err != nil {
		return "", err
	}
	return localBase, nil
}

func (s *Syncer) walk(ctx context.Context, root string) ([]model.Entry, error) {
	var entries []model.Entry
	queue := []string{root}

	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]

		children, err := s.Client.ListDir(ctx, dir)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			entries = append(entries, child)
			if child.Type == model.EntryFolder {
				queue = append(queue, child.Path)
			}
		}
	}

	return entries, nil
}

func (s *Syncer) ensureDir(path string, opts Options) error {
	if opts.DryRun {
		ui.Infof("mkdir %s", path)
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func (s *Syncer) syncFile(ctx context.Context, entry model.Entry, dest string, opts Options, state *SyncState) (syncOutcome, error) {
	if state == nil {
		return outcomeSkipped, errors.New("missing sync state")
	}
	if state.Files == nil {
		state.Files = map[string]FileState{}
	}

	last, hasState := state.Files[entry.Path]

	info, err := os.Stat(dest)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return outcomeSkipped, err
	}
	localExists := err == nil && !info.IsDir()
	if err == nil && info.IsDir() {
		ui.Errorf("skip %s: local path is a directory", dest)
		return outcomeSkipped, nil
	}

	if !localExists {
		return s.downloadAndTrack(ctx, entry, dest, opts, state)
	}

	if !hasState {
		if localMatchesRemote(entry, info) {
			updateState(state, entry, info)
			return outcomeSkipped, nil
		}
		ui.Errorf("conflict %s: local file exists with no prior sync state", dest)
		return outcomeConflict, nil
	}

	remoteChanged := remoteEntryChanged(entry, last)
	localChanged := localEntryChanged(info, last)

	if localChanged && remoteChanged {
		ui.Errorf("conflict %s: local and remote changed since last sync", dest)
		return outcomeConflict, nil
	}

	if localChanged && !remoteChanged {
		ui.Errorf("conflict %s: local changed since last sync", dest)
		return outcomeConflict, nil
	}

	if remoteChanged {
		return s.downloadAndTrack(ctx, entry, dest, opts, state)
	}

	updateState(state, entry, info)
	return outcomeSkipped, nil
}

func (s *Syncer) downloadAndTrack(ctx context.Context, entry model.Entry, dest string, opts Options, state *SyncState) (syncOutcome, error) {
	if opts.DryRun {
		ui.Infof("download %s -> %s", entry.Path, dest)
		updateState(state, entry, fileInfoFromEntry(entry))
		return outcomeDownloaded, nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return outcomeSkipped, err
	}

	tempFile := dest + ".part"
	out, err := os.Create(tempFile)
	if err != nil {
		return outcomeSkipped, err
	}

	if err := s.Client.DownloadFile(ctx, entry.Path, out); err != nil {
		out.Close()
		_ = os.Remove(tempFile)
		return outcomeSkipped, err
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tempFile)
		return outcomeSkipped, err
	}

	if err := os.Rename(tempFile, dest); err != nil {
		_ = os.Remove(tempFile)
		return outcomeSkipped, err
	}

	mtime := entry.UpdatedAt
	if !mtime.IsZero() {
		_ = os.Chtimes(dest, time.Now(), mtime)
	}

	info, err := os.Stat(dest)
	if err != nil {
		return outcomeSkipped, err
	}
	updateState(state, entry, info)
	ui.Infof("downloaded %s", dest)
	return outcomeDownloaded, nil
}

func localMatchesRemote(entry model.Entry, info os.FileInfo) bool {
	if info == nil {
		return false
	}
	if entry.Size > 0 && info.Size() != entry.Size {
		return false
	}
	if !entry.UpdatedAt.IsZero() && info.ModTime().Unix() != entry.UpdatedAt.Unix() {
		return false
	}
	return true
}

func localEntryChanged(info os.FileInfo, last FileState) bool {
	if info == nil {
		return false
	}
	if last.LocalSize != 0 && info.Size() != last.LocalSize {
		return true
	}
	if last.LocalMtime != 0 && info.ModTime().Unix() != last.LocalMtime {
		return true
	}
	return false
}

func remoteEntryChanged(entry model.Entry, last FileState) bool {
	if entry.Chash != "" && entry.Chash != last.RemoteChash {
		return true
	}
	if entry.Mhash != "" && entry.Mhash != last.RemoteMhash {
		return true
	}
	if last.RemoteSize != 0 && entry.Size != last.RemoteSize {
		return true
	}
	if last.RemoteMtime != 0 && entry.UpdatedAt.Unix() != last.RemoteMtime {
		return true
	}
	return false
}

func updateState(state *SyncState, entry model.Entry, info os.FileInfo) {
	if state.Files == nil {
		state.Files = map[string]FileState{}
	}
	state.Files[entry.Path] = FileState{
		RemoteChash: entry.Chash,
		RemoteMhash: entry.Mhash,
		RemoteMtime: entry.UpdatedAt.Unix(),
		RemoteSize:  entry.Size,
		LocalMtime:  info.ModTime().Unix(),
		LocalSize:   info.Size(),
	}
}

func fileInfoFromEntry(entry model.Entry) os.FileInfo {
	return entryFileInfo{
		size:  entry.Size,
		mtime: entry.UpdatedAt,
	}
}

func relPath(root, full string) string {
	root = strings.TrimSuffix(root, "/")
	if root == "" || root == "/" {
		return strings.TrimPrefix(full, "/")
	}
	trimmed := strings.TrimPrefix(full, root)
	return strings.TrimPrefix(trimmed, "/")
}

func writeReport(path string, report syncReport) error {
	if path == "" {
		return errors.New("report path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

type entryFileInfo struct {
	size  int64
	mtime time.Time
}

func (f entryFileInfo) Name() string {
	return ""
}

func (f entryFileInfo) Size() int64 {
	return f.size
}

func (f entryFileInfo) Mode() os.FileMode {
	return 0
}

func (f entryFileInfo) ModTime() time.Time {
	return f.mtime
}

func (f entryFileInfo) IsDir() bool {
	return false
}

func (f entryFileInfo) Sys() any {
	return nil
}
