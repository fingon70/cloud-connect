package model

import "time"

type EntryType string

const (
	EntryFile   EntryType = "file"
	EntryFolder EntryType = "folder"
)

type Entry struct {
	ID        string
	Name      string
	Path      string
	Type      EntryType
	Size      int64
	UpdatedAt time.Time
	Chash     string
	Mhash     string
	Nhash     string
}
