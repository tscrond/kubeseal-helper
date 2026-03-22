# kubeseal-helper

A CLI tool that simplifies creating [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) for Kubernetes. It wraps `kubectl` and `kubeseal` to generate sealed secret YAML files either interactively or from a declarative configuration file.

## Prerequisites

- [`kubectl`](https://kubernetes.io/docs/tasks/tools/) — configured with access to your cluster
- [`kubeseal`](https://github.com/bitnami-labs/sealed-secrets#installation) — the Sealed Secrets CLI

## Installation

### Build from source

```sh
git clone https://github.com/tscrond/kubeseal-helper.git
cd kubeseal-helper
go build -o kubeseal-helper .
```

Or install directly:

```sh
go install github.com/tscrond/kubeseal-helper@latest
```

## Usage

```
kubeseal-helper [command] [flags]
```

### Commands

| Command       | Description                                          |
|---------------|------------------------------------------------------|
| `interactive` | Create and seal a secret using a TUI wizard          |
| `from-file`   | Create and seal secrets from a declarative YAML file |

### Global flags

| Flag       | Description                                                          |
|------------|----------------------------------------------------------------------|
| `--config` | Path to config file (default: `$HOME/.kubeseal-helper.yaml`)         |

---

## `interactive`

Launch a step-by-step terminal wizard that prompts you for all required values.

```sh
kubeseal-helper interactive
```

The wizard will ask for:
1. Number of secret variables
2. Secret name
3. Namespace (selected from live cluster namespaces)
4. For each variable: key name, input method (literal value or file), and the value
5. Secret type (`Opaque` or `kubernetes.io/basic-auth`)

The sealed secret is written to `<secret-name>.yaml` in the current directory.

---

## `from-file`

Create one or more sealed secrets from a declarative YAML file.

```sh
kubeseal-helper from-file -f secrets.yaml
```

### Flag

| Flag           | Short | Required | Description                          |
|----------------|-------|----------|--------------------------------------|
| `--file`       | `-f`  | Yes      | Path to the declarative secrets file |

### Secrets file format

```yaml
# Optional: override kubeContext / controller settings for this file
kubeContext: "my-context"
controllerName: "sealed-secrets"
controllerNamespace: "kube-system"

secrets:
  - name: my-secret
    namespace: "default"
    type: Opaque          # Optional, defaults to Opaque
    data:
      - key: API_KEY
        value: "my-api-key"                  # literal value

      - key: DB_PASSWORD
        valueFromEnv: DB_PASSWORD            # read from environment variable

      - key: tls.crt
        valueFromFile: "/path/to/tls.crt"   # read from file (absolute or relative to the YAML file)
```

Each data entry must define **exactly one** of `value`, `valueFromEnv`, or `valueFromFile`.

Each secret is written to `<secret-name>.yaml` in the current working directory.

### Example

See [examples/secret-example.yaml](examples/secret-example.yaml) for a full example:

```yaml
kubeContext: "bl-prod"
controllerName: "sealed-secrets"
controllerNamespace: "kube-system"
secrets:
  - name: secret1
    namespace: "apps"
    type: Opaque
    data:
      - key: GOOGLE_COOKIE_SECRET
        value: "cookie-secret"
      - key: DB_SECRET
        value: "asodfpasdf"
  - name: secret2
    namespace: "apps"
    type: Opaque
    data:
      - key: bucket-config.json
        valueFromFile: "/home/user/bucket-auth.json"
```

```sh
kubeseal-helper from-file -f examples/secret-example.yaml
# Sealed secret written to secret1.yaml
# Sealed secret written to secret2.yaml
```

---

## Configuration

Settings can be provided via a config file, environment variables, or the secrets YAML file itself (for `from-file`). Precedence: secrets YAML file > environment variables > config file > defaults.

### Config file

Place a file at `~/.kubeseal-helper.yaml`:

```yaml
kubeContext: "my-context"
controllerName: "sealed-secrets"
controllerNamespace: "kube-system"
```

### Environment variables

| Variable                              | Setting              |
|---------------------------------------|----------------------|
| `KUBESEAL_HELPER_KUBE_CONTEXT`        | `kubeContext`        |
| `KUBE_CONTEXT`                        | `kubeContext`        |
| `KUBESEAL_HELPER_CONTROLLER_NAME`     | `controllerName`     |
| `SEALED_SECRETS_CONTROLLER_NAME`      | `controllerName`     |
| `KUBESEAL_HELPER_CONTROLLER_NAMESPACE`| `controllerNamespace`|
| `SEALED_SECRETS_CONTROLLER_NAMESPACE` | `controllerNamespace`|

### Defaults

| Setting               | Default          |
|-----------------------|------------------|
| `controllerName`      | `sealed-secrets` |
| `controllerNamespace` | `kube-system`    |
| Secret type           | `Opaque`         |

## License

MIT — see [LICENSE](LICENSE).
