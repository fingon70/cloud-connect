package sync

import (
	"context"
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
	DryRun bool
}

type Syncer struct {
	Client *api.Client
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

	entries, err := s.walk(ctx, remotePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		rel := relPath(remotePath, entry.Path)
		if rel == "" {
			continue
		}
		localPath := filepath.Join(localRoot, rel)
		switch entry.Type {
		case model.EntryFolder:
			if err := s.ensureDir(localPath, opts); err != nil {
				return err
			}
		case model.EntryFile:
			if err := s.syncFile(ctx, entry, localPath, opts); err != nil {
				return err
			}
		}
	}

	return nil
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

func (s *Syncer) syncFile(ctx context.Context, entry model.Entry, dest string, opts Options) error {
	if !shouldDownload(entry, dest) {
		return nil
	}

	if opts.DryRun {
		ui.Infof("download %s -> %s", entry.Path, dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	tempFile := dest + ".part"
	out, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	if err := s.Client.DownloadFile(ctx, entry.Path, out); err != nil {
		out.Close()
		_ = os.Remove(tempFile)
		return err
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	if err := os.Rename(tempFile, dest); err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	mtime := entry.UpdatedAt
	if !mtime.IsZero() {
		_ = os.Chtimes(dest, time.Now(), mtime)
	}

	ui.Infof("downloaded %s", dest)
	return nil
}

func shouldDownload(entry model.Entry, dest string) bool {
	info, err := os.Stat(dest)
	if err != nil {
		return true
	}
	if info.IsDir() {
		return true
	}
	if entry.Size > 0 && info.Size() != entry.Size {
		return true
	}
	if !entry.UpdatedAt.IsZero() && info.ModTime().Before(entry.UpdatedAt) {
		return true
	}
	return false
}

func relPath(root, full string) string {
	root = strings.TrimSuffix(root, "/")
	if root == "" || root == "/" {
		return strings.TrimPrefix(full, "/")
	}
	trimmed := strings.TrimPrefix(full, root)
	return strings.TrimPrefix(trimmed, "/")
}
