package stats

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/wzshiming/diskchart/internal/scan"
)

// ExtStat is the total size and file count of one extension.
type ExtStat struct {
	Ext, Category string
	Size, Count   int64
}

// ExtReport is the extension breakdown of a tree.
type ExtReport struct {
	Items []ExtStat
	Other ExtStat
	Total int64
}

// ByExtension groups non-directory nodes by lowercase extension, keeping the topK largest groups and folding the rest into Other.
func ByExtension(t *scan.Tree, topK int) ExtReport {
	r := ExtReport{Items: []ExtStat{}, Other: ExtStat{Category: "other"}}
	if t == nil {
		return r
	}
	groups := map[string]*ExtStat{}
	walk(t, func(n scan.Node, _ string) {
		if n.IsDir {
			return
		}
		ext := extOf(n.Name)
		g := groups[ext]
		if g == nil {
			g = &ExtStat{Ext: ext, Category: CategoryOf(ext)}
			groups[ext] = g
		}
		g.Size += n.Size
		g.Count++
		r.Total += n.Size
	})
	for _, g := range groups {
		r.Items = append(r.Items, *g)
	}
	slices.SortFunc(r.Items, func(a, b ExtStat) int {
		if c := cmp.Compare(b.Size, a.Size); c != 0 {
			return c
		}
		return strings.Compare(a.Ext, b.Ext)
	})
	if topK > 0 && len(r.Items) > topK {
		for _, g := range r.Items[topK:] {
			r.Other.Size += g.Size
			r.Other.Count += g.Count
		}
		r.Items = r.Items[:topK]
	}
	return r
}

func extOf(name string) string {
	e := strings.ToLower(filepath.Ext(name))
	if e == "" || strings.EqualFold(e, name) {
		return ""
	}
	return e[1:]
}

var categories = func() map[string]string {
	m := map[string]string{}
	for _, c := range [][2]string{
		{"images", "jpg jpeg png gif webp heic heif svg bmp tif tiff raw cr2 nef arw dng psd ai ico"},
		{"video", "mp4 mov mkv avi webm m4v wmv flv mpg mpeg ts m2ts"}, // .ts: MPEG transport stream outweighs TypeScript sources by size
		{"audio", "mp3 flac wav aac m4a ogg opus wma aiff alac"},
		{"archives", "zip tar gz tgz bz2 xz zst 7z rar dmg iso img pkg jar war"},
		{"code", "go js mjs tsx jsx py java c h cc cpp hpp rs rb php swift kt kts cs css scss html htm json yaml yml toml xml sh bash zsh sql proto lua dart vue svelte md"},
		{"documents", "pdf doc docx xls xlsx ppt pptx txt rtf odt ods odp pages numbers key epub csv tex"},
		{"apps", "app exe dll so dylib msi deb rpm apk ipa appimage bin wasm"},
	} {
		for e := range strings.FieldsSeq(c[1]) {
			if _, dup := m[e]; !dup {
				m[e] = c[0]
			}
		}
	}
	return m
}()

// CategoryOf maps a lowercase, dotless extension to images, video, audio, archives, code, documents, apps, or other.
func CategoryOf(ext string) string {
	if c, ok := categories[ext]; ok {
		return c
	}
	return "other"
}
