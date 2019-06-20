package credentials

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretForDockerRegistryGenerate(t *testing.T) {

	username, password, email, server := "testuser", "testPassword", "test@example.com", "quay.io"
	secretName := "test-secret"
	jsonSecret, _ := handleDockerCfgJSONContent(username, password, email, server)
	cases := []struct {
		name       string
		secretName string
		userName   string
		password   string
		email      string
		server     string
		expected   *corev1.Secret
	}{
		{
			name:       "test-generate-dockerconfigjson-secret",
			userName:   username,
			password:   password,
			email:      email,
			server:     server,
			secretName: secretName,
			expected: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: jsonSecret,
				},
				Type: corev1.SecretTypeDockerConfigJson,
			},
		},
	}

	for i, c := range cases {

		t.Run(c.name, func(t *testing.T) {
			result, _ := GenerateDockerJsonSecret(c.secretName, c.server, c.userName, c.password, c.email)

			if !reflect.DeepEqual(result, c.expected) {
				t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.expected, result)
			}

		})
	}

}
