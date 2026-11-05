package kubernetes_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/bootstrap/kubernetes"
	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

var _ = Describe("Component args reach templates", func() {
	const componentConfig = `components:
  - name: demo
    path:
      - kubernetes/components/demo/values.yaml.tmpl
    args:
      - kubernetes/components/demo/args.yaml
`
	const componentArgsDef = `version: 1.0.0
parameters:
  components:
    demo:
      imageTag:
        type: string
        default: ""
      hostedDomain:
        type: string
        default: "example.com"
      ingressEnabled:
        type: boolean
        default: true
      replicas:
        type: integer
        default: 1
      retentionPeriod:
        type: string
        default: "168h"
`
	// The template reads lowerCamelCase and PascalCase names and uses both
	// forms of default.
	const componentTemplate = `tag: "{{ .imageTag }}"
domain: {{ default "example.com" .hostedDomain }}
domainPascal: {{ .HostedDomain }}
ingress: {{ .ingressEnabled | default true }}
replicas: {{ default 1 .replicas }}
retention: {{ .retentionPeriod | default "168h" }}
namespace: {{ .Namespace | default "demo" }}
`

	var (
		templateDir string
		workspace   string
	)

	BeforeEach(func() {
		root := GinkgoT().TempDir()
		templateDir = filepath.Join(root, "template")
		workspace = filepath.Join(root, "workspace")
		files := map[string]string{
			"kubernetes/config.yaml":                      componentConfig,
			"kubernetes/components/demo/args.yaml":        componentArgsDef,
			"kubernetes/components/demo/values.yaml.tmpl": componentTemplate,
		}
		for rel, content := range files {
			p := filepath.Join(templateDir, rel)
			Expect(os.MkdirAll(filepath.Dir(p), 0o755)).To(Succeed())
			Expect(os.WriteFile(p, []byte(content), 0o644)).To(Succeed())
		}
	})

	render := func(parameters map[string]interface{}) string {
		loader := template.NewLoader(templateDir)
		kubernetesConfig, err := loader.LoadKubernetesConfig()
		Expect(err).NotTo(HaveOccurred())

		templateArgs := renderer.ArgsData{
			"kubernetes": map[string]interface{}{
				"projects": []interface{}{
					map[string]interface{}{
						"name": "prd",
						"components": []interface{}{
							map[string]interface{}{"name": "1-demo", "parameters": parameters},
						},
					},
				},
			},
		}
		data := kubernetes.PrepareKubernetesData(config.GlobalConfig{Workspace: workspace}, "prd")
		Expect(kubernetes.InitializeComponents(kubernetesConfig, loader, templateArgs, workspace, data, "prd", true)).To(Succeed())

		out, err := os.ReadFile(filepath.Join(workspace, "kubernetes", "prd", "1-demo", "values.yaml"))
		Expect(err).NotTo(HaveOccurred())
		return string(out)
	}

	It("uses kebab-case args from default.yaml under every spelling the template reads", func() {
		out := render(map[string]interface{}{
			"image-tag":        "v2.0.1",
			"hosted-domain":    "corp.example",
			"ingress-enabled":  false,
			"replicas":         0,
			"retention-period": "720h",
			"namespace":        "demo-prd",
		})
		Expect(out).To(Equal(`tag: "v2.0.1"
domain: corp.example
domainPascal: corp.example
ingress: false
replicas: 0
retention: 720h
namespace: demo-prd
`))
	})

	It("falls back to args.yaml defaults and template literals when args are not set", func() {
		out := render(map[string]interface{}{})
		Expect(out).To(Equal(`tag: ""
domain: example.com
domainPascal: example.com
ingress: true
replicas: 1
retention: 168h
namespace: demo
`))
	})
})
