package volumes

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/shirou/gopsutil/v4/disk"
)

func stubPartitions(t *testing.T, ps []disk.PartitionStat, err error) {
	t.Helper()
	orig := partitions
	t.Cleanup(func() { partitions = orig })
	partitions = func(ctx context.Context, all bool) ([]disk.PartitionStat, error) {
		if !all {
			t.Errorf("partitions called with all=false, want true")
		}
		return ps, err
	}
}

func stubUsage(t *testing.T, stats map[string]*disk.UsageStat) {
	t.Helper()
	orig := usage
	t.Cleanup(func() { usage = orig })
	usage = func(ctx context.Context, mp string) (*disk.UsageStat, error) {
		if s, ok := stats[mp]; ok {
			return s, nil
		}
		return nil, errors.New("usage failed for " + mp)
	}
}

func TestKeep(t *testing.T) {
	cases := []struct {
		goos, mp, fstype string
		want             bool
	}{
		{"darwin", "/", "apfs", true},
		{"darwin", "/System/Volumes/Data", "apfs", false},
		{"darwin", "/System/Volumes/Preboot", "apfs", false},
		{"darwin", "/Volumes/Backup", "apfs", true},
		{"darwin", "/dev", "devfs", false},
		{"darwin", "/private/var/vm", "apfs", false},
		{"darwin", "/Volumes/X", "", false},
		{"darwin", "/Volumes/T", "TMPFS", false},
		{"darwin", "/Volumes/.timemachine/abc", "apfs", false},
		{"darwin", "/Volumes/com.apple.TimeMachine.localsnapshots/2026", "apfs", false},
		{"darwin", "/Library/Developer/CoreSimulator/Volumes/iOS_22", "apfs", false},
		{"linux", "/", "ext4", true},
		{"linux", "/home", "xfs", true},
		{"linux", "/snap/core", "squashfs", false},
		{"linux", "/var/lib/docker/overlay2/x", "overlay", false},
		{"linux", "/boot/efi", "vfat", false},
		{"linux", "/run", "TMPFS", false},
		{"linux", "/proc", "proc", false},
		{"linux", "/mnt/x", "", false},
		{"windows", "C:", "NTFS", true},
		{"windows", "D:", "FAT32", true},
		{"windows", "E:", "", false},
		{"windows", "F:", "TmpFs", false},
		{"other", "/x", "zfs", true},
		{"other", "/System/Volumes/Data", "apfs", true},
	}
	for _, c := range cases {
		p := disk.PartitionStat{Device: "dev", Mountpoint: c.mp, Fstype: c.fstype}
		if got := keep(c.goos, p); got != c.want {
			t.Errorf("keep(%q, {Mountpoint:%q Fstype:%q}) = %v, want %v", c.goos, c.mp, c.fstype, got, c.want)
		}
	}
}

func TestLabel(t *testing.T) {
	cases := []struct{ goos, mp, want string }{
		{"darwin", "/", ""},
		{"linux", "/", ""},
		{"darwin", "/Volumes/Backup", "Backup"},
		{"linux", "/home", "home"},
		{"windows", "C:\\", ""},
		{"windows", "C:", ""},
		{"windows", "D:\\Data", "Data"},
	}
	for _, c := range cases {
		if got := label(c.goos, c.mp); got != c.want {
			t.Errorf("label(%q, %q) = %q, want %q", c.goos, c.mp, got, c.want)
		}
	}
}

// listFixture returns mountpoints kept on the current GOOS: two good, one whose usage fails, plus a tmpfs to filter.
func listFixture() (fstype, a, b, fail, pseudo string) {
	if runtime.GOOS == "windows" {
		return "NTFS", "C:\\", "D:\\Data", "E:\\", "F:\\"
	}
	fs := "ext4"
	if runtime.GOOS == "darwin" {
		fs = "apfs"
	}
	return fs, "/", "/data/x", "/opt/fail", "/run"
}

func TestList(t *testing.T) {
	fs, a, b, fail, pseudo := listFixture()
	stubPartitions(t, []disk.PartitionStat{
		{Device: "devB", Mountpoint: b, Fstype: fs},
		{Device: "devP", Mountpoint: pseudo, Fstype: "tmpfs"},
		{Device: "devF", Mountpoint: fail, Fstype: fs},
		{Device: "devA", Mountpoint: a, Fstype: fs},
	}, nil)
	stubUsage(t, map[string]*disk.UsageStat{
		a:      {Path: a, Total: 1000, Used: 400, Free: 600},
		b:      {Path: b, Total: 2000, Used: 500, Free: 1500},
		pseudo: {Path: pseudo, Total: 9, Used: 9, Free: 0},
	})

	got, err := List(context.Background())
	if err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
	want := []Volume{
		{Mountpoint: a, Device: "devA", FSType: fs, Label: label(runtime.GOOS, a), Total: 1000, Used: 400, Free: 600},
		{Mountpoint: b, Device: "devB", FSType: fs, Label: label(runtime.GOOS, b), Total: 2000, Used: 500, Free: 1500},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List (filtered, usage-failure skipped, sorted by Mountpoint) =\n%+v\nwant\n%+v", got, want)
	}
	wantLabels := map[string]string{"/": "", "/data/x": "x", "C:\\": "", "D:\\Data": "Data"}
	for _, v := range got {
		if v.Label != wantLabels[v.Mountpoint] {
			t.Errorf("List: Volume %q Label = %q, want %q", v.Mountpoint, v.Label, wantLabels[v.Mountpoint])
		}
	}
}

func TestListAllUsageFail(t *testing.T) {
	fs, a, b, _, _ := listFixture()
	stubPartitions(t, []disk.PartitionStat{
		{Device: "devA", Mountpoint: a, Fstype: fs},
		{Device: "devB", Mountpoint: b, Fstype: fs},
	}, nil)
	stubUsage(t, nil)

	if got, err := List(context.Background()); err == nil {
		t.Errorf("List with every usage failing: err = nil (volumes %+v), want error", got)
	}
}

func TestListPartitionsError(t *testing.T) {
	sentinel := errors.New("partitions boom")
	stubPartitions(t, nil, sentinel)
	stubUsage(t, nil)

	if _, err := List(context.Background()); !errors.Is(err, sentinel) {
		t.Errorf("List with partitions error: err = %v, want wrapping %v", err, sentinel)
	}
}

func TestListEmpty(t *testing.T) {
	stubPartitions(t, nil, nil)
	stubUsage(t, nil)

	got, err := List(context.Background())
	if err != nil {
		t.Errorf("List with no partitions: err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("List with no partitions = %+v, want empty", got)
	}
}

func TestMountpoints(t *testing.T) {
	var ps []disk.PartitionStat
	var want []string
	if runtime.GOOS == "windows" {
		ps = []disk.PartitionStat{
			{Mountpoint: "D:\\Data", Fstype: "NTFS"},
			{Mountpoint: "C:\\", Fstype: "NTFS"},
			{Mountpoint: "D:\\Data", Fstype: "NTFS"},
			{Mountpoint: "F:\\", Fstype: "tmpfs"},
		}
		want = []string{"C:\\", "D:\\Data", "F:\\"}
	} else {
		ps = []disk.PartitionStat{
			{Mountpoint: "/proc", Fstype: "proc"},
			{Mountpoint: "/b//c", Fstype: "ext4"},
			{Mountpoint: "/a/", Fstype: "ext4"},
			{Mountpoint: "/", Fstype: "ext4"},
			{Mountpoint: "/a", Fstype: "ext4"},
			{Mountpoint: "/dev", Fstype: "devfs"},
		}
		want = []string{"/", "/a", "/b/c", "/dev", "/proc"}
	}
	for i := range want {
		want[i] = filepath.Clean(want[i])
	}
	stubPartitions(t, ps, nil)

	got, err := Mountpoints(context.Background())
	if err != nil {
		t.Fatalf("Mountpoints: unexpected error %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Mountpoints (cleaned, deduped, sorted, pseudo included) = %q, want %q", got, want)
	}
}

func TestMountpointsNormalizeDriveLetter(t *testing.T) {
	stubPartitions(t, []disk.PartitionStat{
		{Mountpoint: "C:", Fstype: "NTFS"},
		{Mountpoint: "D:\\", Fstype: "NTFS"},
		{Mountpoint: "/", Fstype: "apfs"},
	}, nil)

	got, err := Mountpoints(context.Background())
	if err != nil {
		t.Fatalf("Mountpoints: unexpected error %v", err)
	}
	for _, want := range []string{"C:\\", "D:\\"} {
		if !slices.Contains(got, want) {
			t.Errorf("Mountpoints = %q, want it to contain %q (bare drive letter normalized)", got, want)
		}
	}
	if want := filepath.Clean("/"); !slices.Contains(got, want) {
		t.Errorf("Mountpoints = %q, want it to contain %q (non-drive mountpoint only cleaned)", got, want)
	}
	for _, bad := range []string{"C:", "C:."} {
		if slices.Contains(got, bad) {
			t.Errorf("Mountpoints = %q, must not contain %q", got, bad)
		}
	}
}

func TestListNormalizesDriveLetterMountpoint(t *testing.T) {
	p := disk.PartitionStat{Device: "devC", Mountpoint: "C:", Fstype: "NTFS"}
	if !keep(runtime.GOOS, p) {
		t.Skipf("partition %+v not kept on %s", p, runtime.GOOS)
	}
	stubPartitions(t, []disk.PartitionStat{p}, nil)
	var gotPaths []string
	orig := usage
	t.Cleanup(func() { usage = orig })
	usage = func(ctx context.Context, mp string) (*disk.UsageStat, error) {
		gotPaths = append(gotPaths, mp)
		return &disk.UsageStat{Path: mp, Total: 100, Used: 40, Free: 60}, nil
	}

	got, err := List(context.Background())
	if err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
	if len(got) != 1 || got[0].Mountpoint != "C:\\" {
		t.Fatalf("List with partition %q = %+v, want one Volume with Mountpoint %q", "C:", got, "C:\\")
	}
	if want := []string{"C:\\"}; !reflect.DeepEqual(gotPaths, want) {
		t.Errorf("usage called with %q, want %q", gotPaths, want)
	}
	// label is GOOS-specific; only windows treats "C:\" as a drive root.
	if want := label(runtime.GOOS, "C:\\"); got[0].Label != want {
		t.Errorf("Volume.Label = %q, want %q", got[0].Label, want)
	}
	if runtime.GOOS == "windows" && got[0].Label != "" {
		t.Errorf("Volume.Label = %q, want empty on windows", got[0].Label)
	}
}

func TestListReal(t *testing.T) {
	ctx := context.Background()
	vols, err := List(ctx)
	if err != nil {
		t.Fatalf("List on real system: %v", err)
	}
	if len(vols) == 0 {
		t.Fatal("List on real system returned no volumes")
	}
	var anyCap bool
	for _, v := range vols {
		if v.Total > 0 {
			anyCap = true
		}
	}
	if !anyCap {
		t.Errorf("List on real system: no volume with Total > 0: %+v", vols)
	}

	mps, err := Mountpoints(ctx)
	if err != nil {
		t.Fatalf("Mountpoints on real system: %v", err)
	}
	if len(mps) == 0 {
		t.Error("Mountpoints on real system returned none")
	}
}
