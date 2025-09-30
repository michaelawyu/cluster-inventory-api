package credentials

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdapilatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
)

const (
	execExtensionName              = "client.authentication.k8s.io/exec"
	additionalCLIArgsExtensionName = "multicluster.x-k8s.io/clusterprofiles/auth/exec/additional-args"
	additionalEnvVarsExtensionName = "multicluster.x-k8s.io/clusterprofiles/auth/exec/additional-envs"
)

type Provider struct {
	Name                            string                   `json:"name"`
	ExecConfig                      *clientcmdapi.ExecConfig `json:"execConfig"`
	AllowAdditionalCLIArgsExtension bool                     `json:"allowAdditionalCLIArgsExtension,omitempty"`
	AllowAdditionalEnvVarsExtension bool                     `json:"allowAdditionalEnvVarsExtension,omitempty"`
}

type CredentialsProvider struct {
	Providers []Provider `json:"providers"`
}

func New(providers []Provider) *CredentialsProvider {
	return &CredentialsProvider{
		Providers: providers,
	}
}

// SetupProviderFileFlag defines the -clusterprofile-provider-file command-line flag and returns a pointer
// to the string that will hold the path. flag.Parse() must still be called manually by the caller
func SetupProviderFileFlag() *string {
	return flag.String("clusterprofile-provider-file", "clusterprofile-provider-file.json", "Path to the JSON configuration file")
}

func NewFromFile(path string) (*CredentialsProvider, error) {
	// 1. Read the file's content
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	// 2. Create a new Providers instance and unmarshal the data into it
	var providers CredentialsProvider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credential proviers: %w", err)
	}

	// 3. Return the populated credentials
	return &providers, nil
}

func (cp *CredentialsProvider) BuildConfigFromCP(clusterprofile *v1alpha1.ClusterProfile) (*rest.Config, error) {
	// 1. obtain the correct provider from the CP
	provider := cp.getProviderFromClusterProfile(clusterprofile)
	if provider == nil {
		return nil, fmt.Errorf("no matching provider found for cluster profile %q", clusterprofile.Name)
	}

	// 2. Get Exec Config
	execConfig, allowAdditionalCLIArgsExtension, allowAdditionalEnvVarsExtension := cp.getExecConfigAndExtFlagsFromConfig(provider.Name)
	if execConfig == nil {
		return nil, fmt.Errorf("no exec credentials found for provider %q", provider.Name)
	}

	// Retrieve the extensions.
	ic := clientcmdapi.NewCluster()
	// The conversion will save the extension data as runtime.Unknown objects.
	if err := clientcmdapilatest.Scheme.Convert(&provider.Cluster, ic, nil); err != nil {
		return nil, fmt.Errorf("failed to convert v1 Cluster to internal: %w", err)
	}
	execExts := ic.Extensions[execExtensionName]

	// Check if the additional CLI args extension exists.
	for idx := range provider.Cluster.Extensions {
		ext := &provider.Cluster.Extensions[idx]

		switch {
		case allowAdditionalCLIArgsExtension && ext.Name == additionalCLIArgsExtensionName:
			var additionalArgs []string
			if err := yaml.Unmarshal(ext.Extension.Raw, &additionalArgs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal additional CLI args extension: %w", err)
			}
			execConfig.Args = append(execConfig.Args, additionalArgs...)
		case allowAdditionalEnvVarsExtension && ext.Name == additionalEnvVarsExtensionName:
			var additionalEnvs map[string]string
			if err := yaml.Unmarshal(ext.Extension.Raw, &additionalEnvs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal additional env vars extension: %w", err)
			}

			// Update the value of existing env vars.
			for idx := range execConfig.Env {
				env := &execConfig.Env[idx]
				if _, exists := additionalEnvs[env.Name]; exists {
					env.Value = additionalEnvs[env.Name]
					delete(additionalEnvs, env.Name)
				}
			}

			// Add new env vars.
			for name, value := range additionalEnvs {
				execConfig.Env = append(execConfig.Env, clientcmdapi.ExecEnvVar{
					Name:  name,
					Value: value,
				})
			}
		}
	}

	// 3. build resulting rest.Config
	config := &rest.Config{
		Host: provider.Cluster.Server,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: provider.Cluster.CertificateAuthorityData,
		},
		Proxy: func(request *http.Request) (*url.URL, error) {
			if provider.Cluster.ProxyURL == "" {
				return nil, nil
			}
			return url.Parse(provider.Cluster.ProxyURL)
		},
	}

	config.ExecProvider = &clientcmdapi.ExecConfig{
		APIVersion:         execConfig.APIVersion,
		Command:            execConfig.Command,
		Args:               execConfig.Args,
		Env:                execConfig.Env,
		InteractiveMode:    "Never",
		ProvideClusterInfo: execConfig.ProvideClusterInfo,
		Config:             execExts,
	}

	return config, nil
}

func (cp *CredentialsProvider) getExecConfigAndExtFlagsFromConfig(providerName string) (*clientcmdapi.ExecConfig, bool, bool) {
	for _, provider := range cp.Providers {
		if provider.Name == providerName {
			return provider.ExecConfig, provider.AllowAdditionalCLIArgsExtension, provider.AllowAdditionalEnvVarsExtension
		}
	}
	return nil, false, false
}

func (cp *CredentialsProvider) getProviderFromClusterProfile(cluster *v1alpha1.ClusterProfile) *v1alpha1.CredentialProvider {
	cpProviderTypes := map[string]*v1alpha1.CredentialProvider{}

	for _, provider := range cluster.Status.CredentialProviders {
		newProvider := provider.DeepCopy()
		cpProviderTypes[provider.Name] = newProvider
	}

	// we return the first provider that the CP supports.
	for _, providerType := range cp.Providers {
		if provider, found := cpProviderTypes[providerType.Name]; found {
			return provider
		}
	}
	return nil
}
