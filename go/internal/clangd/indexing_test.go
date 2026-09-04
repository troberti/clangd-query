package clangd

import "testing"

func TestIsBackgroundIndexProgress(t *testing.T) {
	tests := []struct {
		name     string
		progress ProgressParams
		want     bool
	}{
		{
			name: "background index token",
			progress: ProgressParams{
				Token: "backgroundIndexProgress",
				Value: ProgressValue{Kind: "begin", Title: "indexing"},
			},
			want: true,
		},
		{
			name: "indexing title",
			progress: ProgressParams{
				Token: "other",
				Value: ProgressValue{Kind: "begin", Title: "indexing"},
			},
			want: true,
		},
		{
			name: "unrelated progress",
			progress: ProgressParams{
				Token: "formatting",
				Value: ProgressValue{Kind: "begin", Title: "Formatting"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBackgroundIndexProgress(tt.progress); got != tt.want {
				t.Fatalf("isBackgroundIndexProgress() = %v, want %v", got, tt.want)
			}
		})
	}
}
