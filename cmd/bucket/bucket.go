package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/IxDay/internal/minio"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	connectionSecret string
)

// CreateKubernetesClient creates a Kubernetes clientset using the kubeconfig
// from environment variables or falls back to in-cluster config
func CreateKubernetesClient() (*kubernetes.Clientset, error) {
	// Try to get kubeconfig path from environment variable
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		// Fallback to default location if KUBECONFIG is not set
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %v", err)
		}
		kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	// Try loading the config from the kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		// If loading kubeconfig fails, try in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create config: %v", err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	return clientset, nil
}

func CreateMinioClient() (*minio.BucketClient, error) {
	client, err := CreateKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		b, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("failed to read namespace file: %w", err)
		}
		if b == nil {
			return nil, fmt.Errorf("failed to retrieve current namespace: %w",
				errors.New("no file, no env var"))
		}
		namespace = string(b)
	}
	secret, err := client.CoreV1().Secrets(namespace).
		Get(context.Background(), connectionSecret, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("failed to retrieve connection strings secret: %q\n", err)
		os.Exit(1)
	}
	return minio.NewMinioClientFromSecret(secret)
}

func main() {
	flag.StringVar(&connectionSecret, "connection-secret", "minio-controller-secret", "name of a secret containing connections strings to a minio cluster")
	flag.Parse()
	client, err := CreateMinioClient()
	if err != nil {
		fmt.Printf("failed to instanciate minio client: %q\n", err)
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		fmt.Printf("the command need at least one bucket name")
		os.Exit(1)
	}
	for _, bucket := range os.Args[1:] {
		policy, err := client.GetBucketPolicy(context.Background(), bucket)
		if err != nil {
			fmt.Printf("failed to retrieve bucket policy: %q\n", err)
			os.Exit(1)
		}
		buffer := bytes.Buffer{}
		json.Indent(&buffer, []byte(policy), "", "  ")
		buffer.WriteTo(os.Stdout)
	}

}
