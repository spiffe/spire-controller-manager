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

func LoadClusterStaticEntryFile(path string, scheme *runtime.Scheme, expandEnv bool) (*ClusterStaticEntry, error) {
	var entry ClusterStaticEntry
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file at %s: %w", path, err)
	}

	if expandEnv {
		content = []byte(os.ExpandEnv(string(content)))
	}

	codecs := serializer.NewCodecFactory(scheme)

	// Regardless of if the bytes are of any external version,
	// it will be read successfully and converted into the internal version
	if err = runtime.DecodeInto(codecs.UniversalDecoder(), content, &entry); err != nil {
		return nil, fmt.Errorf("could not decode file (%s) into runtime.Object: %w", path, err)
	}

	return &entry, nil
}

func ListClusterStaticEntries(_ context.Context, manifestPath string, expandEnv bool) ([]ClusterStaticEntry, error) {
	scheme := runtime.NewScheme()
	res := make([]ClusterStaticEntry, 0)
	files, err := os.ReadDir(manifestPath)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}
		fullfile := path.Join(manifestPath, file.Name())
		entry, err := LoadClusterStaticEntryFile(fullfile, scheme, expandEnv)
		if err != nil {
			return nil, err
		}
		// Ignore files of the wrong type in manifestPath
		if entry.APIVersion != "spire.spiffe.io/v1alpha1" || entry.Kind != "ClusterStaticEntry" {
			continue
		}
		res = append(res, *entry)
	}
	return res, nil
}
