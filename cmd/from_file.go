package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	fromFilePath string
	envReader    = viper.New()
)

type secretsFileConfig struct {
	KubeContext         string       `mapstructure:"kubeContext"`
	ControllerName      string       `mapstructure:"controllerName"`
	ControllerNamespace string       `mapstructure:"controllerNamespace"`
	Secrets             []secretSpec `mapstructure:"secrets"`
}

type secretSpec struct {
	Name      string            `mapstructure:"name"`
	Type      string            `mapstructure:"type"`
	Namespace string            `mapstructure:"namespace"`
	Data      []secretDataEntry `mapstructure:"data"`
}

type secretDataEntry struct {
	Key           string `mapstructure:"key"`
	Value         string `mapstructure:"value"`
	ValueFromFile string `mapstructure:"valueFromFile"`
	ValueFromEnv  string `mapstructure:"valueFromEnv"`
}

var fromFileCmd = &cobra.Command{
	Use:   "from-file",
	Short: "Create and seal secrets from a declarative YAML file",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFromFile()
	},
}

func init() {
	envReader.AutomaticEnv()

	rootCmd.AddCommand(fromFileCmd)
	fromFileCmd.Flags().StringVarP(&fromFilePath, "file", "f", "", "Path to declarative secrets YAML file")
	_ = fromFileCmd.MarkFlagRequired("file")
}

func runFromFile() error {
	config, err := loadSecretsConfig(fromFilePath)
	if err != nil {
		return err
	}

	if len(config.Secrets) == 0 {
		return fmt.Errorf("no secrets defined in %s", fromFilePath)
	}

	applyRuntimeConfig(config)

	baseDir := filepath.Dir(fromFilePath)
	for idx, secret := range config.Secrets {
		name := strings.TrimSpace(secret.Name)
		if name == "" {
			return fmt.Errorf("secrets[%d].name is required", idx)
		}

		namespace, err := resolveSecretNamespace(secret.Namespace, idx)
		if err != nil {
			return err
		}

		secretType := strings.TrimSpace(secret.Type)
		if secretType == "" {
			secretType = defaultSecretType
		}

		secretArgs, err := buildSecretArgs(secret.Data, idx, baseDir)
		if err != nil {
			return err
		}

		outputFile, err := createAndSealSecret(name, namespace, secretType, secretArgs)
		if err != nil {
			return err
		}

		fmt.Printf("Sealed secret written to %s\n", outputFile)
	}

	return nil
}

func loadSecretsConfig(path string) (secretsFileConfig, error) {
	loader := viper.New()
	loader.SetConfigFile(path)
	loader.SetConfigType("yaml")

	if err := loader.ReadInConfig(); err != nil {
		return secretsFileConfig{}, fmt.Errorf("failed reading secrets config %s: %w", path, err)
	}

	var config secretsFileConfig
	if err := loader.Unmarshal(&config); err != nil {
		return secretsFileConfig{}, fmt.Errorf("failed parsing secrets config %s: %w", path, err)
	}

	return config, nil
}

func applyRuntimeConfig(config secretsFileConfig) {
	if strings.TrimSpace(config.KubeContext) != "" {
		viper.Set("kubeContext", strings.TrimSpace(config.KubeContext))
	}
	if strings.TrimSpace(config.ControllerName) != "" {
		viper.Set("controllerName", strings.TrimSpace(config.ControllerName))
	}
	if strings.TrimSpace(config.ControllerNamespace) != "" {
		viper.Set("controllerNamespace", strings.TrimSpace(config.ControllerNamespace))
	}
}

func resolveSecretNamespace(secretNamespace string, secretIndex int) (string, error) {
	namespace := strings.TrimSpace(secretNamespace)
	if namespace == "" {
		return "", fmt.Errorf("secrets[%d].namespace is required", secretIndex)
	}

	return namespace, nil
}

func buildSecretArgs(data []secretDataEntry, secretIndex int, baseDir string) ([]string, error) {
	secretArgs := make([]string, 0, len(data))

	for dataIndex, item := range data {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return nil, fmt.Errorf("secrets[%d].data[%d].key is required", secretIndex, dataIndex)
		}

		sources := 0
		if strings.TrimSpace(item.Value) != "" {
			sources++
		}
		if strings.TrimSpace(item.ValueFromFile) != "" {
			sources++
		}
		if strings.TrimSpace(item.ValueFromEnv) != "" {
			sources++
		}

		if sources != 1 {
			return nil, fmt.Errorf("secrets[%d].data[%d] must define exactly one of value, valueFromFile, valueFromEnv", secretIndex, dataIndex)
		}

		if strings.TrimSpace(item.ValueFromFile) != "" {
			filePath := strings.TrimSpace(item.ValueFromFile)
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(baseDir, filePath)
			}

			if _, err := os.Stat(filePath); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("❌ File %q does not exist. Exiting.", filePath)
				}
				return nil, fmt.Errorf("failed to read file %q: %w", filePath, err)
			}

			secretArgs = append(secretArgs, fmt.Sprintf("--from-file=%s=%s", key, filePath))
			continue
		}

		if strings.TrimSpace(item.ValueFromEnv) != "" {
			envName := strings.TrimSpace(item.ValueFromEnv)
			if !envReader.IsSet(envName) {
				return nil, fmt.Errorf("environment variable %q referenced in secrets[%d].data[%d] is not set", envName, secretIndex, dataIndex)
			}

			secretArgs = append(secretArgs, fmt.Sprintf("--from-literal=%s=%s", key, envReader.GetString(envName)))
			continue
		}

		secretArgs = append(secretArgs, fmt.Sprintf("--from-literal=%s=%s", key, item.Value))
	}

	return secretArgs, nil
}
