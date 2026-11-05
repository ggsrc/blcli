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

func TestRender_default(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data map[string]interface{}
		want string
	}{
		{name: "string set, prefix form", tmpl: `{{ default "d" .v }}`, data: map[string]interface{}{"v": "x"}, want: "x"},
		{name: "string set, pipe form", tmpl: `{{ .v | default "d" }}`, data: map[string]interface{}{"v": "x"}, want: "x"},
		{name: "empty string, prefix form", tmpl: `{{ default "d" .v }}`, data: map[string]interface{}{"v": ""}, want: "d"},
		{name: "empty string, pipe form", tmpl: `{{ .v | default "d" }}`, data: map[string]interface{}{"v": ""}, want: "d"},
		{name: "bool false is kept, prefix form", tmpl: `{{ default true .v }}`, data: map[string]interface{}{"v": false}, want: "false"},
		{name: "bool false is kept, pipe form", tmpl: `{{ .v | default true }}`, data: map[string]interface{}{"v": false}, want: "false"},
		{name: "bool true", tmpl: `{{ default false .v }}`, data: map[string]interface{}{"v": true}, want: "true"},
		{name: "int 0 is kept, prefix form", tmpl: `{{ default 1 .v }}`, data: map[string]interface{}{"v": 0}, want: "0"},
		{name: "int 0 is kept, pipe form", tmpl: `{{ .v | default 1 }}`, data: map[string]interface{}{"v": 0}, want: "0"},
		{name: "int set", tmpl: `{{ default 1 .v }}`, data: map[string]interface{}{"v": 3}, want: "3"},
		{name: "nil value, prefix form", tmpl: `{{ default "d" .v }}`, data: map[string]interface{}{"v": nil}, want: "d"},
		{name: "nil value, pipe form", tmpl: `{{ .v | default "d" }}`, data: map[string]interface{}{"v": nil}, want: "d"},
		{name: "missing key, prefix form", tmpl: `{{ default "d" .v }}`, data: map[string]interface{}{}, want: "d"},
		{name: "missing key, pipe form", tmpl: `{{ .v | default "d" }}`, data: map[string]interface{}{}, want: "d"},
		{name: "chain takes first set value", tmpl: `{{ .a | default .b | default "c" }}`, data: map[string]interface{}{"a": "A", "b": "B"}, want: "A"},
		{name: "chain falls back to second value", tmpl: `{{ .a | default .b | default "c" }}`, data: map[string]interface{}{"b": "B"}, want: "B"},
		{name: "chain falls back to literal", tmpl: `{{ .a | default .b | default "c" }}`, data: map[string]interface{}{}, want: "c"},
		{name: "default inside if keeps false", tmpl: `{{ if default true .v }}on{{ else }}off{{ end }}`, data: map[string]interface{}{"v": false}, want: "off"},
		{name: "default inside if uses literal when unset", tmpl: `{{ if default true .v }}on{{ else }}off{{ end }}`, data: map[string]interface{}{}, want: "on"},
		{name: "list value is kept", tmpl: `{{ range default "" .v }}{{ . }}{{ end }}`, data: map[string]interface{}{"v": []interface{}{"a", "b"}}, want: "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.tmpl, tt.data)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Render(%s) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestDefaultFunc(t *testing.T) {
	if got := defaultFunc("d", "x"); got != "x" {
		t.Errorf(`defaultFunc("d", "x") = %v, want "x"`, got)
	}
	if got := defaultFunc("d", ""); got != "d" {
		t.Errorf(`defaultFunc("d", "") = %v, want "d"`, got)
	}
	if got := defaultFunc("d", nil); got != "d" {
		t.Errorf(`defaultFunc("d", nil) = %v, want "d"`, got)
	}
	if got := defaultFunc(true, false); got != false {
		t.Errorf("defaultFunc(true, false) = %v, want false", got)
	}
	if got := defaultFunc(1, 0); got != 0 {
		t.Errorf("defaultFunc(1, 0) = %v, want 0", got)
	}
}

func TestRenderWithArgs_default(t *testing.T) {
	args := map[string]interface{}{
		"IstioVersion":          "1.31.3",
		"IngressGatewayEnabled": false,
	}
	got, err := RenderWithArgs(`tag: {{ .IstioVersion | default "1.27.0" }} enabled: {{ .IngressGatewayEnabled | default true }} type: {{ .IngressGatewayServiceType | default "LoadBalancer" }}`, nil, args)
	if err != nil {
		t.Fatalf("RenderWithArgs() error = %v", err)
	}
	want := "tag: 1.31.3 enabled: false type: LoadBalancer"
	if got != want {
		t.Errorf("RenderWithArgs() = %q, want %q", got, want)
	}
}
