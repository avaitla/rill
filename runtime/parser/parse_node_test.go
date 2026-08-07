package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeYAMLKnownFields(t *testing.T) {
	dst := &struct {
		Known  string `yaml:"known"`
		Custom string `yaml:"custom_thing"`
		Nested []*struct {
			Name   string `yaml:"name"`
			Custom string `yaml:"custom_nested"`
		} `yaml:"nested"`
	}{}

	// Unknown fields prefixed with "custom_" are allowed anywhere,
	// but are still decoded into fields that dst defines (e.g. in custom Rill distributions).
	err := decodeYAMLKnownFields(`
known: a
custom_thing: b
custom_unknown: c
nested:
- name: d
  custom_nested: e
  custom_unknown: f
`, dst)
	require.NoError(t, err)
	require.Equal(t, "a", dst.Known)
	require.Equal(t, "b", dst.Custom)
	require.Len(t, dst.Nested, 1)
	require.Equal(t, "d", dst.Nested[0].Name)
	require.Equal(t, "e", dst.Nested[0].Custom)

	// Unknown fields without the "custom_" prefix return an error
	err = decodeYAMLKnownFields(`unknown: x`, &struct{}{})
	require.ErrorContains(t, err, "field unknown not found")

	// Empty YAML decodes to nothing
	err = decodeYAMLKnownFields(``, &struct{}{})
	require.NoError(t, err)
}
