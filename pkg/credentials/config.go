package credentials

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
)

const (
	execPluginAdditionalArgsExtensionName = "multicluster.x-k8s.io/clusterprofiles/auth/exec/additional-args"
	execPluginAdditionalEnvsExtensionName = "multicluster.x-k8s.io/clusterprofiles/auth/exec/additional-envs"
)

type Provider struct {
	Name       string                   `json:"name"`
	ExecConfig *clientcmdapi.ExecConfig `json:"execConfig"`
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
	// Note that this function will ignore the extensions set in the Cluster struct.
	cluster := convertCluster(provider.Cluster)

	// 2. Get Exec Config
	execConfig := cp.getExecConfigFromConfig(provider.Name)
	if execConfig == nil {
		return nil, fmt.Errorf("no exec credentials found for provider %q", provider.Name)
	}

	// 3. build resulting rest.Config
	config := &rest.Config{
		Host: cluster.Server,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: cluster.CertificateAuthorityData,
		},
		Proxy: func(request *http.Request) (*url.URL, error) {
			if cluster.ProxyURL == "" {
				return nil, nil
			}
			return url.Parse(cluster.ProxyURL)
		},
	}

	expandedArgs := execConfig.Args
	additionalArgs, err := extractAdditionalArgsFromProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to extract additional args from extension: %w", err)
	}
	expandedArgs = append(expandedArgs, additionalArgs...)

	expendedEnvs := execConfig.Env
	additionalEnvs, err := extractAdditionalEnvsFromProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to extract additional envs from extension: %w", err)
	}
	for k, v := range additionalEnvs {
		expendedEnvs = append(expendedEnvs, clientcmdapi.ExecEnvVar{Name: k, Value: v})
	}

	config.ExecProvider = &clientcmdapi.ExecConfig{
		APIVersion:         execConfig.APIVersion,
		Command:            execConfig.Command,
		Args:               expandedArgs,
		Env:                execConfig.Env,
		InteractiveMode:    "Never",
		ProvideClusterInfo: execConfig.ProvideClusterInfo,
	}

	return config, nil
}

func (cp *CredentialsProvider) getExecConfigFromConfig(providerName string) *clientcmdapi.ExecConfig {
	for _, provider := range cp.Providers {
		if provider.Name == providerName {
			return provider.ExecConfig
		}
	}
	return nil
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

func convertCluster(cluster clientcmdv1.Cluster) *clientauthentication.Cluster {
	return &clientauthentication.Cluster{
		Server:                cluster.Server,
		TLSServerName:         cluster.TLSServerName,
		InsecureSkipTLSVerify: cluster.InsecureSkipTLSVerify,
		//CertificateAuthority:     cluster.CertificateAuthority,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
		ProxyURL:                 cluster.ProxyURL,
		DisableCompression:       cluster.DisableCompression,
	}
	// Certificate Authority is a file path, so it doesn't apply to us.
	// Extensions is unclear on how we could use it and is not relevant at the moment.
}

func extractAdditionalArgsFromProvider(provider *v1alpha1.CredentialProvider) ([]string, error) {
	var additionalArgs []string

	for idx := range provider.Extensions {
		ext := provider.Extensions[idx]
		if ext.Name == execPluginAdditionalArgsExtensionName {
			if err := yaml.Unmarshal([]byte(ext.Extension.Raw), &additionalArgs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal extension: %w", err)
			}
			break
		}
	}

	return additionalArgs, nil
}

func extractAdditionalEnvsFromProvider(provider *v1alpha1.CredentialProvider) (map[string]string, error) {
	additionalEnvs := map[string]string{}

	for idx := range provider.Extensions {
		ext := provider.Extensions[idx]
		if ext.Name == execPluginAdditionalEnvsExtensionName {
			if err := yaml.Unmarshal([]byte(ext.Extension.Raw), &additionalEnvs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal extension: %w", err)
			}
			break
		}
	}

	return additionalEnvs, nil
}
