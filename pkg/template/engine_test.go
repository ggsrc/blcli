package template

import (
	"testing"
)

func TestRender_replace(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		data    interface{}
		want    string
		wantErr bool
	}{
		{
			name: "replace underscore with hyphen",
			tmpl: `{{ replace .name "_" "-" }}`,
			data: map[string]interface{}{"name": "prd_private"},
			want: "prd-private",
		},
		{
			name: "replace multiple underscores",
			tmpl: `{{ replace .id "_" "-" }}`,
			data: map[string]interface{}{"id": "app_main_subdomain"},
			want: "app-main-subdomain",
		},
		{
			name: "no match unchanged",
			tmpl: `{{ replace .name "_" "-" }}`,
			data: map[string]interface{}{"name": "prd"},
			want: "prd",
		},
		{
			name: "empty string",
			tmpl: `[{{ replace .name "_" "-" }}]`,
			data: map[string]interface{}{"name": ""},
			want: "[]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.tmpl, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplaceFunc(t *testing.T) {
	if got := replaceFunc("a_b_c", "_", "-"); got != "a-b-c" {
		t.Errorf("replaceFunc() = %q, want %q", got, "a-b-c")
	}
	if got := replaceFunc("nodash", "_", "-"); got != "nodash" {
		t.Errorf("replaceFunc() = %q, want %q", got, "nodash")
	}
}
