package overriders

import (
	"fmt"

	"gopkg.in/yaml.v3"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"

	"sigs.k8s.io/cluster-inventory-api/pkg/credentials"
)

const (
	// Known extension names for the Azure exec config overrider.
	azExecPluginAdditionalArgsExtensionName = "multicluster.x-k8s.io/clusterprofiles/auth/exec/kubelogin-additional-args"
	azExecPluginAdditionalEnvsExtensionName = "multicluster.x-k8s.io/clusterprofiles/auth/exec/kubelogin-additional-envs"
)

type AzureKubeLoginExecConfigOverrider struct{}

var _ credentials.ExecConfigOverrider = &AzureKubeLoginExecConfigOverrider{}

func (o *AzureKubeLoginExecConfigOverrider) OverrideExecConfig(execConfig *clientcmdapi.ExecConfig, provider v1alpha1.CredentialProvider) error {
	expandedArgs := execConfig.Args
	additionalArgs, err := extractAdditionalArgsFromProvider(&provider)
	if err != nil {
		return fmt.Errorf("failed to extract additional args from extension: %w", err)
	}
	expandedArgs = append(expandedArgs, additionalArgs...)

	expandedEnvs := execConfig.Env
	additionalEnvs, err := extractAdditionalEnvsFromProvider(&provider)
	if err != nil {
		return fmt.Errorf("failed to extract additional envs from extension: %w", err)
	}
	for k, v := range additionalEnvs {
		expandedEnvs = append(expandedEnvs, clientcmdapi.ExecEnvVar{Name: k, Value: v})
	}

	execConfig.Args = expandedArgs
	execConfig.Env = expandedEnvs

	return nil
}

func extractAdditionalArgsFromProvider(provider *v1alpha1.CredentialProvider) ([]string, error) {
	var additionalArgs []string

	for idx := range provider.Cluster.Extensions {
		ext := provider.Cluster.Extensions[idx]
		if ext.Name == azExecPluginAdditionalArgsExtensionName {
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

	for idx := range provider.Cluster.Extensions {
		ext := provider.Cluster.Extensions[idx]
		if ext.Name == azExecPluginAdditionalEnvsExtensionName {
			if err := yaml.Unmarshal([]byte(ext.Extension.Raw), &additionalEnvs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal extension: %w", err)
			}
			break
		}
	}

	return additionalEnvs, nil
}
