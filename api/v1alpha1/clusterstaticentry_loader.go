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

func LoadClusterStaticEntryFromFile(path string, scheme *runtime.Scheme, entry *ClusterStaticEntry, expandEnv bool) error {
	if err := loadClusterStaticEntryFile(path, scheme, entry, expandEnv); err != nil {
		return err
	}

	return nil
}

func loadClusterStaticEntryFile(path string, scheme *runtime.Scheme, entry *ClusterStaticEntry, expandEnv bool) error {
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

func ListClusterStaticEntries(_ context.Context, scheme *runtime.Scheme, manifestPath string) ([]ClusterStaticEntry, error) {
	res := make([]ClusterStaticEntry, 0)
	expandEnv := false
	files, err := os.ReadDir(manifestPath)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		var entry ClusterStaticEntry
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}
		fullfile := path.Join(manifestPath, file.Name())
		err = LoadClusterStaticEntryFromFile(fullfile, scheme, &entry, expandEnv)
		if entry.APIVersion != "spire.spiffe.io/v1alpha1" || entry.Kind != "ClusterStaticEntry" {
			continue
		}
		if err != nil {
			return nil, err
		}
		res = append(res, entry)
	}
	return res, nil
}
