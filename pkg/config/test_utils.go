package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

// NewTestConfigFromGroups returns a new Config receiving GroupsMap as input
// and returns a *Config and a filename to a file with the content added
func NewTestConfigFileFromGroups(t *testing.T, input GroupsMap) string {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "grafana-proxy-")
	assert.NoError(t, err)

	config := config{
		Groups: input,
	}

	data, err := yaml.Marshal(&config)
	assert.NoError(t, err)

	_, err = tmpFile.Write(data)
	assert.NoError(t, err)
	err = tmpFile.Close()
	assert.NoError(t, err)

	return tmpFile.Name()
}
