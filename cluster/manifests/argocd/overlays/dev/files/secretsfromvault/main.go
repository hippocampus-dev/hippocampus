package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/hashicorp/vault/api"
	"golang.org/x/xerrors"
	"sigs.k8s.io/kustomize/api/kv"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

// data:[<mediatype>][;base64],<data> https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/Data_URLs#syntax
var dataURLRegexp = regexp.MustCompile(`^data:([a-z]+/[a-z0-9.+-]+)?(;base64)?,(.+)$`)

const inClusterBearerTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"

type vaultSecret struct {
	Path string `json:"path" yaml:"path"`
	Key  string `json:"key" yaml:"key"`
}

type vaultRequestBody struct {
	JWT  string `json:"jwt"`
	Role string `json:"role"`
}

type vaultResponseBody struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
}

type plugin struct {
	h                *resmap.PluginHelpers
	types.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec             struct {
		VaultSecrets []vaultSecret `json:"vaultSecrets" yaml:"vaultSecrets"`
		Behavior     *string       `json:"behavior" yaml:"behavior"`
	} `json:"Spec" yaml:"Spec"`
	client *api.Client
}

var KustomizePlugin plugin

func (p *plugin) Config(h *resmap.PluginHelpers, c []byte) error {
	client, err := initializeVaultClient()
	if err != nil {
		return err
	}

	p.client = client
	p.h = h
	return yaml.Unmarshal(c, p)
}

func (p *plugin) Generate() (resmap.ResMap, error) {
	args := types.SecretArgs{}
	args.Name = p.Name
	args.Namespace = p.Namespace
	if p.Spec.Behavior != nil {
		args.Behavior = *p.Spec.Behavior
	}

	for _, vaultSecret := range p.Spec.VaultSecrets {
		path := vaultSecret.Path
		key := vaultSecret.Key

		secret, err := p.client.Logical().Read(path)
		if err != nil {
			return nil, xerrors.Errorf("failed to read secret: %w", err)
		}
		if secret == nil {
			return nil, errors.New("failed to read secret")
		}

		data, ok := secret.Data["data"].(map[string]interface{})
		if !ok {
			return nil, errors.New("failed to cast secret data")
		}

		value, ok := data[key].(string)
		if !ok {
			return nil, errors.New("failed to cast secret value")
		}
		m := dataURLRegexp.FindStringSubmatch(value)
		if m != nil {
			f, err := os.CreateTemp(p.h.Loader().Root(), "")
			if err != nil {
				return nil, xerrors.Errorf("failed to create tmpfile: %w", err)
			}
			defer f.Close()

			base64Encoded := m[2] == ";base64"
			if base64Encoded {
				b, err := base64.StdEncoding.DecodeString(m[3])
				if err != nil {
					return nil, xerrors.Errorf("failed to decode data url: %w", err)
				}
				if _, err := f.Write(b); err != nil {
					return nil, xerrors.Errorf("failed to write data url to tmpfile: %w", err)
				}
			} else {
				if _, err := f.WriteString(m[3]); err != nil {
					return nil, xerrors.Errorf("failed to write data url to tmpfile: %w", err)
				}
			}
			if err := f.Sync(); err != nil {
				return nil, xerrors.Errorf("failed to sync tmpfile: %w", err)
			}

			args.FileSources = append(args.FileSources, fmt.Sprintf("%s=%s", key, f.Name()))
		} else {
			args.LiteralSources = append(args.LiteralSources, fmt.Sprintf("%s=%s", key, value))
		}
	}
	return p.h.ResmapFactory().FromSecretArgs(kv.NewLoader(p.h.Loader(), p.h.Validator()), args)
}

func initializeVaultClient() (*api.Client, error) {
	vaultAddr, ok := os.LookupEnv("VAULT_ADDR")
	if !ok {
		return nil, errors.New("failed to lookup VAULT_ADDR")
	}

	client, err := api.NewClient(&api.Config{
		Address: vaultAddr,
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to initialize vault client: %w", err)
	}

	token, ok := os.LookupEnv("VAULT_TOKEN")
	if !ok {
		b, err := os.ReadFile(inClusterBearerTokenFile)
		if err != nil {
			return nil, xerrors.Errorf("failed to read bearer token file: %w", err)
		}

		vaultRole, ok := lookupProviderSpecificRoleEnv()
		if !ok {
			return nil, errors.New("failed to lookup vault role")
		}

		token, err = getTokenFromVault(vaultAddr, string(b), vaultRole)
		if err != nil {
			return nil, xerrors.Errorf("failed to get token from vault: %w", err)
		}
	}
	client.SetToken(token)

	return client, nil
}

func lookupProviderSpecificRoleEnv() (string, bool) {
	argocdAppName := os.Getenv("ARGOCD_APP_NAME")
	argocdAppNamespace := os.Getenv("ARGOCD_APP_NAMESPACE")
	if argocdAppName != "" && argocdAppNamespace != "" {
		return fmt.Sprintf("%s.%s", argocdAppName, argocdAppNamespace), true
	}

	return os.LookupEnv("VAULT_ROLE")
}

func getTokenFromVault(addr string, bearerToken string, role string) (string, error) {
	b, err := json.Marshal(vaultRequestBody{
		JWT:  bearerToken,
		Role: role,
	})
	if err != nil {
		return "", xerrors.Errorf("failed to marshal vault request body: %w", err)
	}

	response, err := http.Post(fmt.Sprintf("%s%s", addr, "/v1/auth/kubernetes/login"), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return "", xerrors.Errorf("failed to post to vault: %w", err)
	}
	defer response.Body.Close()

	var vaultResponseBody vaultResponseBody
	if err := json.NewDecoder(response.Body).Decode(&vaultResponseBody); err != nil {
		return "", xerrors.Errorf("failed to decode vault response body: %w", err)
	}

	return vaultResponseBody.Auth.ClientToken, nil
}
