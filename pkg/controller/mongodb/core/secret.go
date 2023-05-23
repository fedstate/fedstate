package core

import (
	"encoding/json"

	"encoding/base64"

	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/mgo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type secretUtil byte

var StaticSecretUtil = new(secretUtil)

func (s *secretUtil) GetAuthInfo(secret *corev1.Secret) (user, password string) {
	if secret == nil {
		return "", ""
	}

	return string(secret.Data[mgo.MongoUser]), string(secret.Data[mgo.MongoPassword])
}

func (s *secretUtil) NewDockerRegistrySecret(namespace, secretName, server, username, password string) (*corev1.Secret, error) {
	// dockerCfg := map[string]map[string]string{"index.docker.io/v1/": {"email": "passed-email", "auth": "cGFzc2VkLXVzZXI6cGFzc2VkLXBhc3N3b3Jk"}}
	// dockercfgAuth := credentialprovider.AuthConfig{
	// 	Username: username,
	// 	Password: password,
	// }
	authConfig := map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte(username + ":" + password))}

	dockerCfg := map[string]map[string]string{server: authConfig}

	dockerCfgJSON := map[string]map[string]map[string]string{"auths": dockerCfg}

	// dockercfgContent, err := json.Marshal(dockerCfg)
	// if err != nil {
	// 	return nil, err
	// }
	dockercfgJSONContent, err := json.Marshal(dockerCfgJSON)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
	}
	secret.Data[corev1.DockerConfigJsonKey] = dockercfgJSONContent
	secret.Type = corev1.SecretTypeDockerConfigJson
	return secret, nil
}
