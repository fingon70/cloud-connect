package sync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const stateFileName = "state.json"

type SyncState struct {
	Version int                  `json:"version"`
	Files   map[string]FileState `json:"files"`
}

type FileState struct {
	RemoteChash string `json:"remote_chash"`
	RemoteMhash string `json:"remote_mhash"`
	RemoteMtime int64  `json:"remote_mtime"`
	RemoteSize  int64  `json:"remote_size"`
	LocalMtime  int64  `json:"local_mtime"`
	LocalSize   int64  `json:"local_size"`
}

func LoadState() (*SyncState, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &SyncState{Version: 1, Files: map[string]FileState{}}, nil
		}
		return nil, err
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Files == nil {
		state.Files = map[string]FileState{}
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return &state, nil
}

func SaveState(state *SyncState) error {
	if state == nil {
		return errors.New("state is nil")
	}
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func statePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hidrive-cli", stateFileName), nil
}
