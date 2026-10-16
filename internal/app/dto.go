package app

import (
	"github.com/wzshiming/diskchart/internal/scan"
	"github.com/wzshiming/diskchart/internal/stats"
	"github.com/wzshiming/diskchart/internal/volumes"
)

// Info describes the running application.
type Info struct{ Version, Platform string }

// Volume is a mounted filesystem with capacity in bytes.
type Volume struct {
	Mountpoint, Label, FSType string
	Total, Used, Free         int64
}

// ScanOptions control a scan started from the frontend.
type ScanOptions struct{ SameDevice bool }

// Progress is a scan progress snapshot.
type Progress struct {
	Dirs, Files, Bytes, Errors int64
	CurrentPath                string
	ElapsedMs                  int64
}

// Summary totals a finished scan; Dirs counts directories including the root (== final Progress.Dirs).
type Summary struct{ Size, Files, Dirs, Errors, DurationMs int64 }

// ScanState is the current scan lifecycle state; RootID is -1 without a tree.
type ScanState struct {
	Status, Root string
	RootID       int32
	Progress     Progress
	Summary      Summary
	Error        string
	TreeStale    bool
}

// View is a bounded subtree snapshot; aggregate pseudo-nodes have ID -1.
type View struct {
	ID        int32
	Name      string
	Size      int64
	IsDir     bool
	FileCount int64
	Aggregate bool
	Count     int64
	Children  []View
}

// Crumb is one step on the path from the root to a node.
type Crumb struct {
	ID   int32
	Name string
}

// Entry is one ranked file or directory.
type Entry struct {
	ID            int32
	Name, RelPath string
	Size          int64
	IsDir         bool
	FileCount     int64
	ModTime       int64
}

// ExtStat is the total size and file count of one extension.
type ExtStat struct {
	Ext, Category string
	Size, Count   int64
}

// ExtReport is the extension breakdown of the scanned tree.
type ExtReport struct {
	Items []ExtStat
	Other ExtStat
	Total int64
}

// ScanError is a path that could not be read.
type ScanError struct{ Path, Msg string }

// DeleteResult reports what a successful Delete removed.
type DeleteResult struct {
	ParentID                               int32
	RemovedSize, RemovedFiles, RemovedDirs int64
}

// FinishedEvent is the payload of EventScanFinished.
type FinishedEvent struct {
	Status, Error, Root string
	RootID              int32
	Summary             Summary
}

// TreeChangedEvent is the payload of EventTreeChanged.
type TreeChangedEvent struct{ RemovedID, ParentID int32 }

// EventPayloads exists only so Wails generates TS models for the event payloads.
type EventPayloads struct {
	Progress    Progress
	Finished    FinishedEvent
	TreeChanged TreeChangedEvent
}

func toVolume(v volumes.Volume) Volume {
	return Volume{
		Mountpoint: v.Mountpoint,
		Label:      v.Label,
		FSType:     v.FSType,
		Total:      int64(v.Total),
		Used:       int64(v.Used),
		Free:       int64(v.Free),
	}
}

func toProgress(p scan.Progress) Progress {
	return Progress{
		Dirs:        p.Dirs,
		Files:       p.Files,
		Bytes:       p.Bytes,
		Errors:      p.Errors,
		CurrentPath: p.CurrentPath,
		ElapsedMs:   p.Elapsed.Milliseconds(),
	}
}

func toView(v stats.View) View {
	out := View{
		ID:        int32(v.ID),
		Name:      v.Name,
		Size:      v.Size,
		IsDir:     v.IsDir,
		FileCount: v.FileCount,
		Aggregate: v.Aggregate,
		Count:     v.Count,
		Children:  make([]View, len(v.Children)),
	}
	for i, c := range v.Children {
		out.Children[i] = toView(c)
	}
	return out
}

func toEntries(es []stats.Entry) []Entry {
	out := make([]Entry, len(es))
	for i, e := range es {
		out[i] = Entry{
			ID:        int32(e.ID),
			Name:      e.Name,
			RelPath:   e.RelPath,
			Size:      e.Size,
			IsDir:     e.IsDir,
			FileCount: e.FileCount,
			ModTime:   e.ModTime,
		}
	}
	return out
}

func toExtStat(e stats.ExtStat) ExtStat {
	return ExtStat{Ext: e.Ext, Category: e.Category, Size: e.Size, Count: e.Count}
}
