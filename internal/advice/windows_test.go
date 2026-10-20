//go:build windows

package advice_test

import (
	"testing"

	"github.com/wzshiming/DiskChart/internal/advice"
)

func TestWindowsRules(t *testing.T) {
	const tempDir = `C:\Users\u\AppData\Local\Temp`
	r := advice.New(advice.Config{
		GOOS:        "windows",
		Home:        `C:\Users\u`,
		TempDir:     tempDir,
		UID:         "S-1-5-21-1",
		Mountpoints: []string{`C:\`, `D:\`},
		Getenv: env(map[string]string{
			"SystemRoot":        `C:\Windows`,
			"ProgramFiles":      `C:\Program Files`,
			"ProgramFiles(x86)": `C:\Program Files (x86)`,
			"ProgramData":       `C:\ProgramData`,
			"APPDATA":           `C:\Users\u\AppData\Roaming`,
			"LOCALAPPDATA":      `C:\Users\u\AppData\Local`,
		}),
	})
	runCases(t, r, []classifyCase{
		{path: `C:\`, want: protected(advice.Volume)},
		{path: `c:\`, want: protected(advice.Volume)},
		{path: `E:\`, want: protected(advice.Volume)},
		{path: `\\srv\share`, want: protected(advice.Volume)},
		{path: `\\srv\share\x`, want: none},
		{path: `C:`, want: none},
		{path: `\foo`, want: none},
		{path: `C:/Users/u`, want: protected(advice.Home)},
		{path: `C:\Users`, want: protected(advice.System)},
		{path: `C:\ProgramData`, want: protected(advice.System)},
		{path: `C:\$Recycle.Bin`, want: protected(advice.System)},
		{path: `C:\Windows\Temp`, want: protected(advice.System)},
		{path: `C:\Program Files\nodejs\node_modules`, dir: true, want: protected(advice.System)},
		{path: `C:\Program Files (x86)\x`, want: protected(advice.System)},
		{path: `D:\System Volume Information\x`, want: protected(advice.System)},
		{path: `D:\Recovery\x`, want: protected(advice.System)},
		{path: `C:\ProgramData\x`, want: none},
		{path: `C:\$Recycle.Bin\S-1-5-21-1`, want: none},
		{path: `C:\$Recycle.Bin\S-1-5-21-2\x`, want: none},
		{path: tempDir, want: none},
		{path: `C:\$Recycle.Bin\S-1-5-21-1\$R1.txt`, want: recommended(advice.Trash)},
		{path: `C:\Windows\Temp\x`, want: recommended(advice.Temp)},
		{path: tempDir + `\x`, want: recommended(advice.Temp)},
		{path: `C:\Users\u\AppData`, want: protected(advice.UserData)},
		{path: `C:\Users\u\AppData\Roaming`, want: protected(advice.UserData)},
		{path: `C:\Users\u\AppData\Local`, want: protected(advice.UserData)},
		{path: `C:\Users\u\OneDrive`, want: protected(advice.UserData)},
		{path: `C:\Users\u\AppData\Local\go-build\x`, want: recommended(advice.Cache)},
		{path: `C:\Users\u\AppData\Local\Microsoft\Windows\INetCache\x`, want: recommended(advice.Cache)},
		{path: `C:\Users\u\go\pkg\mod\x`, want: recommended(advice.Rebuildable)},
		{path: `C:\USERS\U`, want: protected(advice.Home)},
	})
}
