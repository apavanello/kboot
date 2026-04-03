package kube

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"kboot/internal/aws"

	"gopkg.in/yaml.v3"
)

type KubeConfig struct {
	APIVersion     string          `yaml:"apiVersion"`
	Kind           string          `yaml:"kind"`
	Clusters       []KClusterEntry `yaml:"clusters"`
	Contexts       []KContextEntry `yaml:"contexts"`
	CurrentContext string          `yaml:"current-context"`
	Users          []KUserEntry    `yaml:"users"`
	Preferences    struct{}        `yaml:"preferences"`
}

type KClusterEntry struct {
	Name    string   `yaml:"name"`
	Cluster KCluster `yaml:"cluster"`
}

type KCluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type KContextEntry struct {
	Name    string   `yaml:"name"`
	Context KContext `yaml:"context"`
}

type KContext struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type KUserEntry struct {
	Name string `yaml:"name"`
	User KUser  `yaml:"user"`
}

type KUser struct {
	Exec KExec `yaml:"exec"`
}

type KExec struct {
	APIVersion string   `yaml:"apiVersion"`
	Command    string   `yaml:"command"`
	Args       []string `yaml:"args"`
	Env        []KEnv   `yaml:"env"`
}

type KEnv struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func Generate(dir string, alias string, info *aws.ClusterInfo, region, profile string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	kbootPath, execErr := os.Executable()
	if execErr != nil {
		kbootPath = "kboot"
	} else if filepath.Base(kbootPath) != "kboot" {
		if looked, err := exec.LookPath("kboot"); err == nil {
			kbootPath = looked
		} else {
			kbootPath = "kboot"
		}
	}

	kc := KubeConfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: alias,
		Clusters: []KClusterEntry{
			{
				Name: info.ARN,
				Cluster: KCluster{
					Server:                   info.Endpoint,
					CertificateAuthorityData: info.CAData,
				},
			},
		},
		Contexts: []KContextEntry{
			{
				Name: alias,
				Context: KContext{
					Cluster: info.ARN,
					User:    alias,
				},
			},
		},
		Users: []KUserEntry{
			{
				Name: alias,
				User: KUser{
					Exec: KExec{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    kbootPath,
						Args: []string{
							"token",
							"--cluster-name", info.Name,
							"--region", region,
							"--profile", profile,
						},
					},
				},
			},
		},
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.yaml", alias))
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	if err := encoder.Encode(kc); err != nil {
		return "", err
	}

	return filename, nil
}
