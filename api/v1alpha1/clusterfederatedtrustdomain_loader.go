package v1alpha1

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func LoadClusterFederatedTrustDomainFromFile(path string, scheme *runtime.Scheme, entry *ClusterFederatedTrustDomain, expandEnv bool) error {
	if err := loadClusterFederatedTrustDomainFile(path, scheme, entry, expandEnv); err != nil {
		return err
	}

	return nil
}

func loadClusterFederatedTrustDomainFile(path string, scheme *runtime.Scheme, entry *ClusterFederatedTrustDomain, expandEnv bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file at %s: %w", path, err)
	}

	if expandEnv {
		content = []byte(os.ExpandEnv(string(content)))
	}

	codecs := serializer.NewCodecFactory(scheme)

	// Regardless of if the bytes are of any external version,
	// it will be read successfully and converted into the internal version
	if err = runtime.DecodeInto(codecs.UniversalDecoder(), content, entry); err != nil {
		return fmt.Errorf("could not decode file into runtime.Object: %w", err)
	}

	return nil
}

func ListClusterFederatedTrustDomains(_ context.Context, scheme *runtime.Scheme, manifestPath string) ([]ClusterFederatedTrustDomain, error) {
	res := make([]ClusterFederatedTrustDomain, 0)
	expandEnv := false
	files, err := os.ReadDir(manifestPath)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		var entry = ClusterFederatedTrustDomain{}
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}
		fullfile := path.Join(manifestPath, file.Name())
		err = LoadClusterFederatedTrustDomainFromFile(fullfile, scheme, &entry, expandEnv)
		if entry.APIVersion != "spire.spiffe.io/v1alpha1" || entry.Kind != "ClusterFederatedTrustDomain" {
			continue
		}
		if err != nil {
			return nil, err
		}
		res = append(res, entry)
	}
	return res, nil
}
