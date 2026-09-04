package clangd

import (
	"runtime"
	"testing"
)

func TestPathFromFileURI_WindowsDrivePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	client := &ClangdClient{ProjectRoot: `C:\tmp\project`}

	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "standard clangd URI",
			uri:  "file:///C:/tmp/project/foo.c",
			want: `C:\tmp\project\foo.c`,
		},
		{
			name: "two-slash clangd URI",
			uri:  "file://C:/tmp/project/foo.c",
			want: `C:\tmp\project\foo.c`,
		},
		{
			name: "URI with spaces encoded",
			uri:  "file:///C:/tmp/my%20project/foo.c",
			want: `C:\tmp\my project\foo.c`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.PathFromFileURI(tt.uri)
			if got != tt.want {
				t.Errorf("PathFromFileURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestFileURIFromPath_RoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	client := &ClangdClient{ProjectRoot: `C:\tmp\project`}
	path := `C:\tmp\project\mcuxsdk\components\edgefast_wifi\source\wpl_nxp.c`

	uri := client.FileURIFromPath(path)
	got := client.PathFromFileURI(uri)

	if got != path {
		t.Errorf("round trip failed: got %q, want %q (uri was %q)", got, path, uri)
	}
}

func TestPathFromFileURI_UnixPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	client := &ClangdClient{ProjectRoot: "/tmp/project"}
	uri := "file:///tmp/project/foo.c"
	want := "/tmp/project/foo.c"

	got := client.PathFromFileURI(uri)
	if got != want {
		t.Errorf("PathFromFileURI(%q) = %q, want %q", uri, got, want)
	}
}

func TestFileURIFromPathProducesValidURI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	client := &ClangdClient{ProjectRoot: `C:\tmp\project`}
	path := `C:\tmp\project\foo.c`
	uri := client.FileURIFromPath(path)
	if uri != "file:///C:/tmp/project/foo.c" {
		t.Errorf("FileURIFromPath(%q) = %q, want file:///C:/tmp/project/foo.c", path, uri)
	}
}
