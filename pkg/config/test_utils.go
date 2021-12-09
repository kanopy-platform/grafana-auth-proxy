package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func NewTestConfigFromString(t *testing.T, input string) *Config {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "grafana-proxy-")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// data, err := yaml.Marshal(&input)
	// assert.NoError(t, err)

	_, err = tmpFile.Write([]byte(input))
	assert.NoError(t, err)
	err = tmpFile.Close()
	assert.NoError(t, err)

	config := New()
	err = config.Load(tmpFile.Name())
	assert.NoError(t, err)

	return config
}
