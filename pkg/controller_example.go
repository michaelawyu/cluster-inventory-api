package main

import (
	"flag"
	"log"

	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	"sigs.k8s.io/cluster-inventory-api/pkg/credentials"
)

func main() {
	credentialsProviders := credentials.SetupProviderFileFlag()
	flag.Parse()

	cpCreds, err := credentials.NewFromFile(*credentialsProviders)
	if err != nil {
		log.Fatalf("Got error reading credentials providers: %v", err)
	}

	// normally we would get this clusterprofile from the local cluster (maybe a watch?)
	// and we would maintain the restconfigs for clusters we're interested in.
	exampleClusterProfile := v1alpha1.ClusterProfile{
		Spec: v1alpha1.ClusterProfileSpec{
			DisplayName: "My Cluster",
		},
		Status: v1alpha1.ClusterProfileStatus{
			CredentialProviders: []v1alpha1.CredentialProvider{
				{
					Name: "gkeFleet",
					Cluster: clientcmdapiv1.Cluster{
						Server: "https://myserver.tld:443",
					},
				},
			},
		},
	}

	restConfigForMyCluster, err := cpCreds.BuildConfigFromCP(&exampleClusterProfile, false, nil)
	if err != nil {
		log.Fatalf("Got error generating restConfig: %v", err)
	}
	log.Printf("Got credentials: %v", restConfigForMyCluster)
	// I can then use this rest.Config to build a k8s client.
}

func WithoutExtensionsSupport() {
	// Typically, for an application to authenticate with a Kubernetes cluster, it would need
	// both cluster-specific instructions and application-specific instructions. For example,
	// in AKS (federated authentication with K8s API server as OIDC issuer), an application needs to know:
	//
	// * connectivity info. (cluster-specific)
	// * an ID token, issued by the K8s API server (application-specific)
	//   The application usually reads the token from a mounted volume.
	// * the client ID for the federated identity (cluster-specific/application-specific)
	// * the tenant ID for the target AKS cluster (cluster-specific)
	// * the authority hostname and the AKS access permission scopes for the target AKS cluster (cluster-specific)
	//
	// With the current API enhancements, if we do not support extensions (KEP 541), aside from
	// connectivity info, there are not many places where cluster-specific information can
	// be put in a cluster profile. And as a result, applications will have to supply the information
	// directly via the Provider struct, which means that each cluster would have its own
	// provider (even if they use the same exec plugin):
	credProviders := &credentials.CredentialsProvider{
		Providers: []credentials.Provider{
			{
				Name: "aks-bravelion",
				ExecConfig: &clientcmdapi.ExecConfig{
					Command: "kubelogin",
					Args: []string{
						"--authority-host", "AUTH_HOST_FOR_BRAVELION",
						"--client-id", "CLIENT_ID_FOR_BRAVELION",
						"--tenant-id", "TENANT_ID_FOR_BRAVELION",
						// The example assumes that the ID token is read from a file.
					},
				},
			},
			{
				Name: "aks-smartfish",
				ExecConfig: &clientcmdapi.ExecConfig{
					Command: "kubelogin",
					Args: []string{
						"--authority-host", "AUTH_HOST_FOR_SMARTFISH",
						"--client-id", "CLIENT_ID_FOR_SMARTFISH",
						"--tenant-id", "TENANT_ID_FOR_SMARTFISH",
						// The example assumes that the ID token is read from a file.
					},
				},
			},
			{
				Name: "aks-jumpingcat",
				ExecConfig: &clientcmdapi.ExecConfig{
					Command: "kubelogin",
					Args: []string{
						"--authority-host", "AUTH_HOST_FOR_JUMPINGCAT",
						"--client-id", "CLIENT_ID_FOR_JUMPINGCAT",
						"--tenant-id", "TENANT_ID_FOR_JUMPINGCAT",
						// The example assumes that the ID token is read from a file.
					},
				},
			},
			// And the list goes on...
			//
			// It is cumbersome and static.
		},
	}

	bravelionCP := v1alpha1.ClusterProfile{
		Spec: v1alpha1.ClusterProfileSpec{
			DisplayName: "bravelion",
		},
		Status: v1alpha1.ClusterProfileStatus{
			CredentialProviders: []v1alpha1.CredentialProvider{
				{
					Name: "aks-bravelion",
					Cluster: clientcmdapiv1.Cluster{
						Server:                   "https://bravelion.aks.cluster:443",
						CertificateAuthorityData: []byte("CERTIFICATE_AUTHORITY_DATA_FOR_BRAVELION"),
					},
				},
			},
		},
	}

	restConfig, err := credProviders.BuildConfigFromCP(&bravelionCP, false, nil)
	if err != nil {
		log.Fatalf("Failed to prepare REST config: %v", err)
	}
	log.Printf("REST config generated: %v", restConfig)

}

func WithExtensionsSupport() {
	// With support for extensions, however, users could place cluster-specific information
	// in the extensions field, which will automatically parsed as arguments to the exec plugin.
	// This saves the trouble of each application needing to track cluster-specific information
	// for all clusters and allows discoveries.
	credProviders := &credentials.CredentialsProvider{
		Providers: []credentials.Provider{
			{
				Name: "aks",
				ExecConfig: &clientcmdapi.ExecConfig{
					Command: "kubelogin",
					Args:    []string{
						// The example assumes that the ID token is read from a file.
					},
				},
			},
		},
	}

	bravelionCP := v1alpha1.ClusterProfile{
		Spec: v1alpha1.ClusterProfileSpec{
			DisplayName: "bravelion",
		},
		Status: v1alpha1.ClusterProfileStatus{
			CredentialProviders: []v1alpha1.CredentialProvider{
				{
					Name: "aks",
					Cluster: clientcmdapiv1.Cluster{
						Server:                   "https://bravelion.aks.cluster:443",
						CertificateAuthorityData: []byte("CERTIFICATE_AUTHORITY_DATA_FOR_BRAVELION"),
						Extensions: []clientcmdapiv1.NamedExtension{
							{
								// This is a name reserved in KEP 541 for per-cluster exec config.
								//
								// We might use a different name.
								Name: "client.authentication.k8s.io/exec",
								Extension: runtime.RawExtension{
									Raw: []byte(`{
										"authority-host": "AUTH_HOST_FOR_BRAVELION",
										"client-id": "CLIENT_ID_FOR_BRAVELION",
										"tenant-id": "TENANT_ID_FOR_BRAVELION"
									}`),
								},
							},
						},
					},
				},
			},
		},
	}

	// rest.Config.ExecProvider field has a child field, Config, which should be sourced
	// from the extension field in the Cluster struct and automatically supplied to
	// the exec plugin when it is called (e.g., by client-go). Unfortunately it seems that
	// at this moment client-go's exec plugin support ignores the field so it is up
	// to us to read the extensions and parse them as arguments to the exec plugin.
	restConfig, err := credProviders.BuildConfigFromCP(&bravelionCP, true, nil)
	if err != nil {
		log.Fatalf("Failed to prepare REST config: %v", err)
	}
	log.Printf("REST config generated: %v", restConfig)
}
