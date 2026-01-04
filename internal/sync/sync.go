package sync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fingon70/cloud-connect/internal/api"
	"github.com/fingon70/cloud-connect/internal/model"
	"github.com/fingon70/cloud-connect/internal/ui"
)

type Options struct {
	DryRun     bool
	Delete     bool
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
	Deleted       int      `json:"deleted"`
	Errors        int      `json:"errors"`
	ConflictPaths []string `json:"conflicts_list,omitempty"`
	ErrorPaths    []string `json:"errors_list,omitempty"`
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
	keep := map[string]struct{}{}
	if opts.Delete {
		keep[localBase] = struct{}{}
	}

	for _, entry := range entries {
		rel := relPath(remotePath, entry.Path)
		if rel == "" {
			continue
		}
		localPath := filepath.Join(localBase, rel)
		switch entry.Type {
		case model.EntryFolder:
			if opts.Delete {
				addKeepPaths(keep, localBase, localPath)
			}
			if err := s.ensureDir(localPath, opts); err != nil {
				report.Errors++
				report.ErrorPaths = append(report.ErrorPaths, entry.Path+": "+err.Error())
				continue
			}
		case model.EntryFile:
			if opts.Delete {
				addKeepPaths(keep, localBase, localPath)
			}
			outcome, err := s.syncFile(ctx, entry, localPath, opts, state)
			if err != nil {
				report.Errors++
				report.ErrorPaths = append(report.ErrorPaths, entry.Path+": "+err.Error())
				continue
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

	if opts.Delete {
		deleted, deleteErrors := s.pruneLocal(localBase, keep, remotePath, state, opts)
		report.Deleted += deleted
		if len(deleteErrors) > 0 {
			report.Errors += len(deleteErrors)
			report.ErrorPaths = append(report.ErrorPaths, deleteErrors...)
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

	ui.Infof("sync summary: downloaded=%d skipped=%d conflicts=%d deleted=%d errors=%d",
		report.Downloaded, report.Skipped, report.Conflicts, report.Deleted, report.Errors)
	if report.Errors > 0 {
		return errors.New("sync completed with errors")
	}
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

func addKeepPaths(keep map[string]struct{}, base, target string) {
	if keep == nil {
		return
	}
	if target == "" {
		return
	}
	if base != "" {
		keep[base] = struct{}{}
	}
	keep[target] = struct{}{}
	parent := filepath.Dir(target)
	for parent != "." && parent != string(filepath.Separator) && parent != base {
		keep[parent] = struct{}{}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}
	if base != "" {
		keep[base] = struct{}{}
	}
}

type deleteCandidate struct {
	path  string
	isDir bool
}

func (s *Syncer) pruneLocal(localBase string, keep map[string]struct{}, remoteRoot string, state *SyncState, opts Options) (int, []string) {
	if localBase == "" {
		return 0, []string{"local base path is empty"}
	}
	var candidates []deleteCandidate
	err := filepath.WalkDir(localBase, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == localBase {
			return nil
		}
		if _, ok := keep[current]; ok {
			return nil
		}
		candidates = append(candidates, deleteCandidate{path: current, isDir: entry.IsDir()})
		return nil
	})
	if err != nil {
		return 0, []string{err.Error()}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i].path) > len(candidates[j].path)
	})

	deleted := 0
	var errs []string
	for _, candidate := range candidates {
		if opts.DryRun {
			ui.Infof("delete %s", candidate.path)
			deleted++
			continue
		}
		if err := os.Remove(candidate.path); err != nil {
			errs = append(errs, candidate.path+": "+err.Error())
			continue
		}
		deleted++
		if !candidate.isDir && state != nil {
			if rel, err := filepath.Rel(localBase, candidate.path); err == nil && rel != "." && rel != "" {
				remotePath := path.Join(remoteRoot, filepath.ToSlash(rel))
				delete(state.Files, remotePath)
			}
		}
	}
	return deleted, errs
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
