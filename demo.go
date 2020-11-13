package main

import (
	"crypto/tls"
	"fmt"
	"net/http"

	qclient "github.com/redhat-cop/quay-openshift-registry-operator/pkg/client/quay"
)

func main() {
	baseURL := "https://example-quayecosystem-quay-quay-enterprise.10.9.51.93.nip.io"
	authToken := "YuXpfsyo677R1viNpjkIc78sx08ldCgnOjxhNbKq"

	t := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{
		Transport: &t,
	}

	setupClient := qclient.NewClient(&httpClient, baseURL, authToken)

	user, _, err := setupClient.GetUser()

	if err.Error != nil {
		fmt.Println(err)
	}

	fmt.Printf("%+v\n", user)
}
