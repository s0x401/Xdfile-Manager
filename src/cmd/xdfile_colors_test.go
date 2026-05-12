package cmd

import "testing"

func TestXdfileResolveFileColorKnownTypes(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		label string
	}{
		{name: "main.go", want: "#6ed8e5", label: "Go source"},
		{name: "photo.png", want: "#5bc0eb", label: "image"},
		{name: "movie.mp4", want: "#b388ff", label: "video"},
		{name: "report.pdf", want: "#d35400", label: "pdf"},
		{name: "Dockerfile", want: "#099cec", label: "full filename alias"},
		{name: "installer.msi", want: "#9b59b6", label: "Windows installer package"},
		{name: "plugin.dll", want: "#ff7844", label: "Windows dynamic library"},
		{name: "profile.ps1", want: "#2ecc71", label: "PowerShell script"},
		{name: "autorun.vbs", want: "#36cfc9", label: "VBScript file"},
		{name: "update.cab", want: "#f3a43b", label: "Windows cabinet archive"},
		{name: "layout.xml", want: "#3498db", label: "XML document"},
	}

	for _, tt := range tests {
		got, ok := xdfileResolveFileColor(tt.name)
		if !ok {
			t.Fatalf("expected %s file %q to resolve to a color", tt.label, tt.name)
		}
		if string(got) != tt.want {
			t.Fatalf("expected %s file %q to use color %q, got %q", tt.label, tt.name, tt.want, got)
		}
	}
}

func TestXdfileResolveFileColorUnknownTypeFallsBack(t *testing.T) {
	if got, ok := xdfileResolveFileColor("notes.unknownext"); ok {
		t.Fatalf("expected unknown extension to fall back to default color, got %q", got)
	}
}
