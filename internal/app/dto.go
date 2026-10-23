package app

import (
	"path/filepath"
	"strings"

	"github.com/wzshiming/DiskChart/internal/advice"
	"github.com/wzshiming/DiskChart/internal/scan"
	"github.com/wzshiming/DiskChart/internal/stats"
	"github.com/wzshiming/DiskChart/internal/volumes"
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

// View is a bounded subtree snapshot; aggregate pseudo-nodes have ID -1 and no advice.
type View struct {
	ID        int32
	Name      string
	Size      int64
	Apparent  int64
	IsDir     bool
	FileCount int64
	Aggregate bool
	Count     int64
	Skip      int64  // aggregate only: index of the first merged child in Children(parent)
	Advice    string // deletion advice: "protected", "recommended" or ""
	Reason    string // why; "" exactly when Advice is ""
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
	Apparent      int64
	IsDir         bool
	FileCount     int64
	ModTime       int64
	Advice        string // deletion advice: "protected", "recommended" or ""
	Reason        string // why; "" exactly when Advice is ""
}

// CleanupReport lists the largest recommended entries; Total and Count cover every recommended node.
type CleanupReport struct {
	Total   int64
	Count   int64
	Entries []Entry
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

// UserRules are the user's own advice rules as stored; paths may start with ~ to mean the home directory.
type UserRules struct {
	Protected   []string // subtrees that must never be deleted
	Recommended []string // directories whose contents are safe to clean up
	Names       []string // directory base names safe to clean up wherever they occur
}

// UserRulesInfo is the current user rules and the file they live in; LoadError says why that file failed to load, "" when it loaded.
type UserRulesInfo struct {
	Rules     UserRules
	Path      string
	LoadError string
}

// RulesChangedEvent is the payload of EventRulesChanged, emitted after the user rules were saved and applied.
type RulesChangedEvent struct {
	Path string // the rules file
}

// EventPayloads exists only so Wails generates TS models for the event payloads.
type EventPayloads struct {
	Progress     Progress
	Finished     FinishedEvent
	TreeChanged  TreeChangedEvent
	RulesChanged RulesChangedEvent
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

// toUserRules copies u with every slice non-nil, so the frontend never sees null.
func toUserRules(u advice.UserRules) UserRules {
	return UserRules{Protected: cloneStrings(u.Protected), Recommended: cloneStrings(u.Recommended), Names: cloneStrings(u.Names)}
}

// cloneStrings returns a non-nil copy of s.
func cloneStrings(s []string) []string { return append([]string{}, s...) }

// adviser attaches deletion advice to the DTOs of one tree.
type adviser struct {
	rules    *advice.Rules
	root     scan.ID // the scan root, always protected
	rootPath string  // Tree.Path(root)
}

// verdict returns the advice and reason for node id at the absolute path; dir says whether it is a directory.
func (ad adviser) verdict(id scan.ID, path string, dir bool) (string, string) {
	if id == ad.root {
		return string(advice.Protected), string(advice.Root)
	}
	v := ad.rules.Classify(path, dir)
	return string(v.Class), string(v.Reason)
}

// childPath joins name beneath parent byte-for-byte as Tree.Path would.
func childPath(parent, name string) string {
	if strings.HasSuffix(parent, string(filepath.Separator)) {
		return parent + name
	}
	return parent + string(filepath.Separator) + name
}

// toView converts v, whose node lives at the absolute path.
func toView(v stats.View, path string, ad adviser) View {
	out := View{
		ID:        int32(v.ID),
		Name:      v.Name,
		Size:      v.Size,
		Apparent:  v.Apparent,
		IsDir:     v.IsDir,
		FileCount: v.FileCount,
		Aggregate: v.Aggregate,
		Count:     v.Count,
		Skip:      v.Skip,
		Children:  make([]View, len(v.Children)),
	}
	if !v.Aggregate {
		out.Advice, out.Reason = ad.verdict(v.ID, path, v.IsDir)
	}
	for i, c := range v.Children {
		out.Children[i] = toView(c, childPath(path, c.Name), ad)
	}
	return out
}

func toEntries(es []stats.Entry, ad adviser) []Entry {
	out := make([]Entry, len(es))
	for i, e := range es {
		out[i] = Entry{
			ID:        int32(e.ID),
			Name:      e.Name,
			RelPath:   e.RelPath,
			Size:      e.Size,
			Apparent:  e.Apparent,
			IsDir:     e.IsDir,
			FileCount: e.FileCount,
			ModTime:   e.ModTime,
		}
		out[i].Advice, out[i].Reason = ad.verdict(e.ID, childPath(ad.rootPath, e.RelPath), e.IsDir)
	}
	return out
}

func toExtStat(e stats.ExtStat) ExtStat {
	return ExtStat{Ext: e.Ext, Category: e.Category, Size: e.Size, Count: e.Count}
}
