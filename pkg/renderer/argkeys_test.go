package renderer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/renderer"
)

var _ = Describe("Arg key aliases", func() {
	DescribeTable("ArgKeyAliases",
		func(key string, want []string) {
			Expect(renderer.ArgKeyAliases(key)).To(Equal(want))
		},
		Entry("kebab-case", "hosted-domain", []string{"HostedDomain", "hostedDomain"}),
		Entry("abbreviation at the end", "root-url", []string{"RootURL", "rootUrl", "rootURL"}),
		Entry("abbreviation at the start", "argocd-url", []string{"ArgoCDURL", "argocdUrl", "argocdURL"}),
		Entry("snake_case", "project_id", []string{"ProjectID", "projectId", "projectID"}),
		Entry("three words", "vmagent-cpu-limit", []string{"VmagentCpuLimit", "vmagentCpuLimit"}),
		Entry("lowerCamelCase", "imageTag", []string{"ImageTag"}),
		Entry("PascalCase", "IstioVersion", []string{"istioVersion"}),
		Entry("leading acronym", "RBACPolicies", []string{}),
		Entry("all caps", "HPA", []string{}),
		Entry("single lower-case word", "namespace", []string{"Namespace"}),
	)

	It("PascalCaseArgKey keeps the existing conversion", func() {
		Expect(renderer.PascalCaseArgKey("project-id")).To(Equal("ProjectID"))
		Expect(renderer.PascalCaseArgKey("secret-manager-service-account-email")).To(Equal("SecretManagerServiceAccountEmail"))
		Expect(renderer.PascalCaseArgKey("istio-version")).To(Equal("IstioVersion"))
		Expect(renderer.PascalCaseArgKey("controller-manifest-url")).To(Equal("ControllerManifestURL"))
	})

	It("CanonicalArgKey matches every spelling of one argument", func() {
		for _, k := range []string{"hosted-domain", "hosted_domain", "hostedDomain", "HostedDomain"} {
			Expect(renderer.CanonicalArgKey(k)).To(Equal("hosteddomain"), k)
		}
		Expect(renderer.CanonicalArgKey("argocd-url")).To(Equal(renderer.CanonicalArgKey("ArgoCDURL")))
	})

	Describe("AddArgAliases", func() {
		It("makes a kebab-case arg readable in PascalCase and lowerCamelCase", func() {
			dst := map[string]interface{}{}
			renderer.AddArgAliases(dst, map[string]interface{}{"hosted-domain": "corp.example"})
			Expect(dst).To(HaveKeyWithValue("hosted-domain", "corp.example"))
			Expect(dst).To(HaveKeyWithValue("hostedDomain", "corp.example"))
			Expect(dst).To(HaveKeyWithValue("HostedDomain", "corp.example"))
		})

		It("never lets an alias replace a key that params spell out", func() {
			dst := map[string]interface{}{}
			renderer.AddArgAliases(dst, map[string]interface{}{
				"image-tag": "from-kebab",
				"imageTag":  "explicit",
			})
			Expect(dst).To(HaveKeyWithValue("imageTag", "explicit"))
			Expect(dst).To(HaveKeyWithValue("image-tag", "from-kebab"))
			Expect(dst).To(HaveKey("ImageTag"))
		})

		It("lets params replace values that dst already had", func() {
			dst := map[string]interface{}{"ProjectID": "from-global"}
			renderer.AddArgAliases(dst, map[string]interface{}{"project-id": "from-component"})
			Expect(dst).To(HaveKeyWithValue("ProjectID", "from-component"))
		})

		It("keeps false and 0", func() {
			out := renderer.WithArgAliases(map[string]interface{}{"auth-enabled": false, "replica-count": 0})
			Expect(out).To(HaveKeyWithValue("authEnabled", false))
			Expect(out).To(HaveKeyWithValue("ReplicaCount", 0))
		})
	})

	Describe("MergeArgDefaults", func() {
		It("lets an args value in any spelling replace the default", func() {
			args := renderer.WithArgAliases(map[string]interface{}{"image-tag": "v2.0.1"})
			merged := renderer.MergeArgDefaults(args, map[string]interface{}{"imageTag": "", "replicaCount": 1})
			Expect(merged).To(HaveKeyWithValue("imageTag", "v2.0.1"))
			Expect(merged).To(HaveKeyWithValue("ImageTag", "v2.0.1"))
			Expect(merged).To(HaveKeyWithValue("image-tag", "v2.0.1"))
			Expect(merged).To(HaveKeyWithValue("replicaCount", 1))
			Expect(merged).To(HaveKeyWithValue("ReplicaCount", 1))
		})

		It("matches spellings the alias rules do not produce", func() {
			// "remote-write-url" yields remoteWriteUrl and remoteWriteURL; a default
			// spelled "RemoteWriteUrl" is still matched by its canonical form.
			merged := renderer.MergeArgDefaults(
				map[string]interface{}{"remote-write-url": "http://vminsert:8480"},
				map[string]interface{}{"RemoteWriteUrl": "http://10.10.0.8:8480"},
			)
			Expect(merged).To(HaveKeyWithValue("RemoteWriteUrl", "http://vminsert:8480"))
			Expect(merged).To(HaveKeyWithValue("remoteWriteUrl", "http://vminsert:8480"))
		})

		It("keeps an explicit false over a true default", func() {
			merged := renderer.MergeArgDefaults(
				renderer.WithArgAliases(map[string]interface{}{"auth-enabled": false}),
				map[string]interface{}{"authEnabled": true},
			)
			Expect(merged).To(HaveKeyWithValue("authEnabled", false))
		})

		It("never changes keys that args already has", func() {
			merged := renderer.MergeArgDefaults(
				map[string]interface{}{"namespace": "custom"},
				map[string]interface{}{"namespace": "default-ns", "Namespace": "default-ns"},
			)
			Expect(merged).To(HaveKeyWithValue("namespace", "custom"))
			Expect(merged).To(HaveKeyWithValue("Namespace", "custom"))
		})
	})
})
