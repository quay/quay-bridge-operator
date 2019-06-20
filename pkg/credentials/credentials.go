package credentials

import (
	"encoding/base64"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateDockerJsonSecret(name string, server string, username string, password string, email string) (*corev1.Secret, error) {

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
	}
	secret.Name = name
	secret.Type = corev1.SecretTypeDockerConfigJson
	secret.Data = map[string][]byte{}

	dockercfgJSONContent, err := handleDockerCfgJSONContent(username, password, email, server)
	if err != nil {
		return nil, err
	}
	secret.Data[corev1.DockerConfigJsonKey] = dockercfgJSONContent

	return secret, err
}

func handleDockerCfgJSONContent(username, password, email, server string) ([]byte, error) {
	dockercfgAuth := DockerConfigEntry{
		Email: email,
		Auth:  encodeDockerConfigFieldAuth(username, password),
	}

	dockerCfgJSON := DockerConfigJSON{
		Auths: map[string]DockerConfigEntry{server: dockercfgAuth},
	}

	return json.Marshal(dockerCfgJSON)
}

func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}

type DockerConfigJSON struct {
	Auths DockerConfig `json:"auths"`
}

type DockerConfig map[string]DockerConfigEntry

type DockerConfigEntry struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"`
}
