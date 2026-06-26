package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/agent"
	"blcli/pkg/cli"
)

var _ = Describe("Agent commands", func() {
	Describe("contract", func() {
		It("prints a filtered JSON contract", func() {
			cmd := cli.NewContractCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs([]string{"apply", "terraform", "--format", "json"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			var contract agent.ToolContract
			Expect(json.Unmarshal(output.Bytes(), &contract)).To(Succeed())
			Expect(contract.SchemaVersion).To(Equal(agent.ToolContractSchemaVersion))
			Expect(contract.Commands).To(HaveLen(1))
			Expect(contract.Commands[0].Name).To(Equal("apply terraform"))
		})

		It("fails for an unknown command filter", func() {
			cmd := cli.NewContractCommand()
			cmd.SetArgs([]string{"missing"})

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown command"))
		})
	})

	Describe("diagnose", func() {
		It("prints JSON diagnosis from a message", func() {
			cmd := cli.NewDiagnoseCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs([]string{"--message", "kubectl failed opening ~/.kube/config.lock", "--format", "json"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			var diagnosis agent.FailureDiagnosis
			Expect(json.Unmarshal(output.Bytes(), &diagnosis)).To(Succeed())
			Expect(diagnosis.SchemaVersion).To(Equal(agent.FailureDiagnosisSchemaVersion))
			Expect(diagnosis.Category).To(Equal("state_lock_conflict"))
		})

		It("can read diagnosis input from a file", func() {
			dir := GinkgoT().TempDir()
			logPath := filepath.Join(dir, "failure.log")
			Expect(os.WriteFile(logPath, []byte("Error: quota exceeded for CPUs"), 0o644)).To(Succeed())

			cmd := cli.NewDiagnoseCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs([]string{"--file", logPath, "--format", "yaml"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("category: quota_exceeded"))
		})

		It("requires exactly one message source", func() {
			cmd := cli.NewDiagnoseCommand()
			cmd.SetArgs([]string{"--message", "one", "two"})

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one"))
		})
	})
})
