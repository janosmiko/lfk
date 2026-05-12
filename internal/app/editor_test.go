package app

import (
	"reflect"
	"runtime"
	"testing"
)

func TestEditorCommand(t *testing.T) {
	platformDefault := "vi"
	if runtime.GOOS == "windows" {
		platformDefault = "notepad"
	}

	tests := []struct {
		name       string
		kubeEditor string
		editor     string
		want       []string
	}{
		{
			name:       "KUBE_EDITOR wins over EDITOR",
			kubeEditor: "nano",
			editor:     "emacs",
			want:       []string{"nano"},
		},
		{
			name:       "EDITOR used when KUBE_EDITOR empty",
			kubeEditor: "",
			editor:     "emacs",
			want:       []string{"emacs"},
		},
		{
			name:       "platform default when both empty",
			kubeEditor: "",
			editor:     "",
			want:       []string{platformDefault},
		},
		{
			name:       "splits editor with flags",
			kubeEditor: "code --wait",
			editor:     "",
			want:       []string{"code", "--wait"},
		},
		{
			name:       "preserves quoted path with spaces",
			kubeEditor: `"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl" --wait`,
			editor:     "",
			want:       []string{"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl", "--wait"},
		},
		{
			name:       "malformed input falls back to platform default",
			kubeEditor: `vim "unterminated`,
			editor:     "",
			want:       []string{platformDefault},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KUBE_EDITOR", tt.kubeEditor)
			t.Setenv("EDITOR", tt.editor)
			got := editorCommand()
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("editorCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
