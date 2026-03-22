package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultSecretType          = "Opaque"
	defaultControllerName      = "sealed-secrets"
	defaultControllerNamespace = "kube-system"
)

func createAndSealSecret(secretName, namespace, secretType string, secretArgs []string) (string, error) {
	if strings.TrimSpace(secretName) == "" {
		return "", fmt.Errorf("secret name cannot be empty")
	}

	if strings.TrimSpace(secretType) == "" {
		secretType = defaultSecretType
	}

	rawSecret, err := renderSecretYAML(secretName, namespace, secretType, secretArgs)
	if err != nil {
		return "", err
	}

	outputFile := fmt.Sprintf("%s.yaml", secretName)
	if err := os.WriteFile(outputFile, rawSecret, 0o644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", outputFile, err)
	}

	controllerName := viper.GetString("controllerName")
	if controllerName == "" {
		controllerName = defaultControllerName
	}

	controllerNamespace := viper.GetString("controllerNamespace")
	if controllerNamespace == "" {
		controllerNamespace = defaultControllerNamespace
	}

	sealArgs := []string{
		"--controller-name", controllerName,
		"--controller-namespace", controllerNamespace,
		"-f", outputFile,
		"-w", outputFile,
	}

	if _, err := runCommand("kubeseal", sealArgs...); err != nil {
		return "", fmt.Errorf("failed to seal secret with kubeseal: %w", err)
	}

	return outputFile, nil
}

func renderSecretYAML(secretName, namespace, secretType string, secretArgs []string) ([]byte, error) {
	createArgs := kubectlContextArgs()
	createArgs = append(createArgs, "create", "secret", "generic", secretName)
	if strings.TrimSpace(namespace) != "" {
		createArgs = append(createArgs, "-n", namespace)
	}

	createArgs = append(createArgs, secretArgs...)
	if secretType != defaultSecretType {
		createArgs = append(createArgs, "--type="+secretType)
	}
	createArgs = append(createArgs, "--dry-run=client", "-o", "yaml")

	rawSecret, err := runCommand("kubectl", createArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret yaml with kubectl: %w", err)
	}

	return rawSecret, nil
}

func listNamespaces() ([]string, error) {
	args := kubectlContextArgs()
	args = append(args, "get", "ns", "-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)

	output, err := runCommand("kubectl", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces via kubectl: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	namespaces := make([]string, 0, len(lines))
	for _, line := range lines {
		ns := strings.TrimSpace(line)
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}

	if len(namespaces) == 0 {
		return nil, fmt.Errorf("kubectl returned no namespaces")
	}

	return namespaces, nil
}

func kubectlContextArgs() []string {
	contextName := strings.TrimSpace(viper.GetString("kubeContext"))
	if contextName == "" {
		return nil
	}

	return []string{"--context", contextName}
}

func runCommand(name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil, err
	}

	return output, nil
}
