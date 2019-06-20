package v1alpha1

import (
	"testing"
)

func TestAllowedNamespaces(t *testing.T) {

	cases := []struct {
		quayIntegration *QuayIntegration
		namespace       string
		expected        bool
	}{
		{
			quayIntegration: &QuayIntegration{
				Spec: QuayIntegrationSpec{},
			},
			namespace: "example",
			expected:  true,
		},
		{
			quayIntegration: &QuayIntegration{
				Spec: QuayIntegrationSpec{},
			},
			namespace: "openshift",
			expected:  false,
		},
		{
			quayIntegration: &QuayIntegration{
				Spec: QuayIntegrationSpec{
					WhitelistNamespaces: []string{
						"openshift",
					},
				},
			},
			namespace: "openshift",
			expected:  true,
		},
		{
			quayIntegration: &QuayIntegration{
				Spec: QuayIntegrationSpec{
					BlacklistNamespaces: []string{
						"example",
					},
				},
			},
			namespace: "example",
			expected:  false,
		},
		{
			quayIntegration: &QuayIntegration{
				Spec: QuayIntegrationSpec{
					BlacklistNamespaces: []string{
						"example",
					},
					WhitelistNamespaces: []string{
						"example",
					},
				},
			},
			namespace: "example",
			expected:  true,
		},
	}

	for i, c := range cases {
		result := c.quayIntegration.IsAllowedNamespace(c.namespace)

		if c.expected != result {
			t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.expected, result)
		}
	}
}
