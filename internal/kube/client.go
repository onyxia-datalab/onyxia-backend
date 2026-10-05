package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	clientset kubernetes.Interface
	config    *rest.Config
}

func NewClient(kubeconfigPath string) (*Client, error) {
	cfg, source, err := loadKubeConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	slog.Info("kube: config loaded", slog.String("source", source))

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kube: clientset: %w", err)
	}
	return &Client{clientset: cs, config: cfg}, nil
}

func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

func (c *Client) Config() *rest.Config {
	return c.config
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// ServerVersion() takes no context, so issue the same request through the
	// REST client to make the timeout effective.
	body, err := c.Clientset().Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Raw()
	if err != nil {
		return fmt.Errorf("kube: API unreachable: %w", err)
	}
	var ver version.Info
	if err := json.Unmarshal(body, &ver); err != nil {
		return fmt.Errorf("kube: decode server version: %w", err)
	}
	slog.InfoContext(ctx, "kube: API reachable", slog.String("server_version", ver.GitVersion))
	return nil
}

func loadKubeConfig(explicit string) (*rest.Config, string, error) {
	if explicit != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", explicit)
		if err != nil {
			return nil, "", fmt.Errorf("kubeconfig(%s): %w", explicit, err)
		}
		return cfg, explicit, nil
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, "in-cluster", nil
	}
	path := os.Getenv("KUBECONFIG")
	if path == "" {
		path = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, "", fmt.Errorf("kubeconfig(%s): %w", path, err)
	}
	return cfg, path, nil
}
