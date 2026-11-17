//go:build gpu

package win32

import (
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
)

// These use the clipboard of the machine, and check it with Windows PowerShell,
// another program, so they run with the GPU tests, locally only. The text on
// the clipboard is put back afterwards; other kinds are lost.

func powershell(t *testing.T, script string) string {
	t.Helper()
	exe, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("no Windows PowerShell")
	}
	script = "[Console]::OutputEncoding = [Text.Encoding]::UTF8; " + script
	out, err := exec.Command(exe, "-NoProfile", "-STA", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("powershell: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestClipboard_OtherProgramsReadAndWriteEveryKind(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "clipboard"})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	saved := w.ClipboardText()
	defer w.SetClipboardText(saved)

	dir := t.TempDir()
	file := filepath.Join(dir, "メモ.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	img.SetNRGBA(1, 1, color.NRGBA{200, 10, 20, 255})
	w.WriteClipboard(platform.ClipboardContent{Text: "和文", HTML: "<b>太字</b>", Image: img, Files: []string{file}})

	if k := w.ClipboardKinds(); k != platform.ClipboardText|platform.ClipboardHTML|platform.ClipboardImage|platform.ClipboardFiles {
		t.Errorf("kinds %b", k)
	}
	got := powershell(t, `Add-Type -AssemblyName System.Windows.Forms
$c = [System.Windows.Forms.Clipboard]
$i = $c::GetImage()
"text=" + $c::GetText()
"files=" + ($c::GetFileDropList() -join ";")
"image=" + $i.Width + "x" + $i.Height + " " + $i.GetPixel(1, 1).R
"html=" + ($c::GetText([System.Windows.Forms.TextDataFormat]::Html) -match "<!--StartFragment--><b>")`)
	for _, want := range []string{"text=和文", "files=" + file, "image=4x3 200", "html=True"} {
		if !strings.Contains(got, want) {
			t.Errorf("PowerShell read %q, want %q in it", got, want)
		}
	}

	// And the other way: what .NET puts there.
	powershell(t, `Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$d = New-Object System.Windows.Forms.DataObject
$b = New-Object System.Drawing.Bitmap 5, 2
$b.SetPixel(4, 1, [System.Drawing.Color]::FromArgb(255, 0, 128, 255))
$d.SetImage($b)
$l = New-Object System.Collections.Specialized.StringCollection
[void]$l.Add("`+file+`")
$d.SetFileDropList($l)
[System.Windows.Forms.Clipboard]::SetDataObject($d, $true)`)
	c := w.ReadClipboard(platform.ClipboardImage | platform.ClipboardFiles)
	if c.Image == nil || c.Image.Rect.Dx() != 5 || c.Image.NRGBAAt(4, 1) != (color.NRGBA{0, 128, 255, 255}) {
		t.Errorf("image from .NET: %v", c.Image)
	}
	if !slices.Equal(c.Files, []string{file}) {
		t.Errorf("files from .NET: %q", c.Files)
	}
}
