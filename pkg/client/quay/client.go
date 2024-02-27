package quay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/quay/quay-bridge-operator/pkg/utils"
)

type QuayClient struct {
	BaseURL    *url.URL
	httpClient *http.Client
	AuthToken  string
}

func (c *QuayClient) GetUser() (User, *http.Response, QuayApiError) {
	req, err := c.newRequest("GET", "/api/v1/user", nil)
	if err != nil {
		return User{}, nil, QuayApiError{Error: err}
	}
	var user User
	resp, err := c.do(req, &user)

	return user, resp, QuayApiError{Error: err}
}

func (c *QuayClient) GetOrganizationByname(orgName string) (Organization, *http.Response, QuayApiError) {
	req, err := c.newRequest("GET", fmt.Sprintf("/api/v1/organization/%s", orgName), nil)
	if err != nil {
		return Organization{}, nil, QuayApiError{Error: err}
	}
	var organization Organization
	resp, err := c.do(req, &organization)

	return organization, resp, QuayApiError{Error: err}
}

func (c *QuayClient) CreateOrganization(name string) (StringValue, *http.Response, QuayApiError) {
	newOrganization := OrganizationRequest{
		Name:  name,
		Email: fmt.Sprintf("%s@redhat.com", name),
	}

	req, err := c.newRequest("POST", "/api/v1/organization/", newOrganization)
	if err != nil {
		return StringValue{}, nil, QuayApiError{Error: err}
	}

	var newOrganizationResponse StringValue
	resp, err := c.do(req, &newOrganizationResponse)

	return newOrganizationResponse, resp, QuayApiError{Error: err}
}

func (c *QuayClient) GetOrganizationRobotAccount(organizationName string, robotName string) (RobotAccount, *http.Response, QuayApiError) {
	req, err := c.newRequest("GET", fmt.Sprintf("/api/v1/organization/%s/robots/%s", organizationName, robotName), nil)
	if err != nil {
		return RobotAccount{}, nil, QuayApiError{Error: err}
	}

	var getOrganizationRobotResponse RobotAccount
	resp, err := c.do(req, &getOrganizationRobotResponse)

	return getOrganizationRobotResponse, resp, QuayApiError{Error: err}
}

func (c *QuayClient) GetPrototypesByOrganization(organizationName string) (PrototypesResponse, *http.Response, QuayApiError) {
	req, err := c.newRequest("GET", fmt.Sprintf("/api/v1/organization/%s/prototypes", organizationName), nil)
	if err != nil {
		return PrototypesResponse{}, nil, QuayApiError{Error: err}
	}

	var getPrototypeResponse PrototypesResponse
	resp, err := c.do(req, &getPrototypeResponse)

	return getPrototypeResponse, resp, QuayApiError{Error: err}
}

func (c *QuayClient) CreateOrganizationRobotAccount(organizationName string, robotName string) (RobotAccount, *http.Response, QuayApiError) {
	req, err := c.newRequest("PUT", fmt.Sprintf("/api/v1/organization/%s/robots/%s", organizationName, robotName), nil)
	if err != nil {
		return RobotAccount{}, nil, QuayApiError{Error: err}
	}

	var createOrganizationRobotResponse RobotAccount
	resp, err := c.do(req, &createOrganizationRobotResponse)

	return createOrganizationRobotResponse, resp, QuayApiError{Error: err}
}

func (c *QuayClient) DeleteOrganization(orgName string) (*http.Response, QuayApiError) {
	req, err := c.newRequest("DELETE", fmt.Sprintf("/api/v1/organization/%s", orgName), nil)
	if err != nil {
		return nil, QuayApiError{Error: err}
	}

	resp, err := c.do(req, nil)

	return resp, QuayApiError{Error: err}
}

func (c *QuayClient) CreateRobotPermissionForOrganization(organizationName string, robotAccount string, role string) (Prototype, *http.Response, QuayApiError) {
	robotOrganizationPermission := Prototype{
		Role: role,
		Delegate: PrototypeDelegate{
			Kind:      "user",
			Name:      robotAccount,
			Robot:     true,
			OrgMember: true,
		},
	}

	req, err := c.newRequest("POST", fmt.Sprintf("/api/v1/organization/%s/prototypes", organizationName), robotOrganizationPermission)
	if err != nil {
		return Prototype{}, nil, QuayApiError{Error: err}
	}

	var newPrototypeResponse Prototype
	resp, err := c.do(req, &newPrototypeResponse)

	return newPrototypeResponse, resp, QuayApiError{Error: err}
}

func (c *QuayClient) GetRepository(orgName string, repositoryName string) (Repository, *http.Response, QuayApiError) {
	req, err := c.newRequest("GET", fmt.Sprintf("/api/v1/repository/%s/%s", orgName, repositoryName), nil)
	if err != nil {
		return Repository{}, nil, QuayApiError{Error: err}
	}

	var repository Repository
	resp, err := c.do(req, &repository)

	return repository, resp, QuayApiError{Error: err}
}

func (c *QuayClient) CreateRepository(namespace, name string) (RepositoryRequest, *http.Response, QuayApiError) {
	newRepository := RepositoryRequest{
		Repository:  name,
		Namespace:   namespace,
		Kind:        "image",
		Visibility:  "private",
		Description: "",
	}

	req, err := c.newRequest("POST", "/api/v1/repository", newRepository)
	if err != nil {
		return RepositoryRequest{}, nil, QuayApiError{Error: err}
	}

	var newRepositoryResponse RepositoryRequest
	resp, err := c.do(req, &newRepositoryResponse)

	return newRepositoryResponse, resp, QuayApiError{Error: err}
}

func (c *QuayClient) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if !utils.IsZeroOfUnderlyingType(c.AuthToken) {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *QuayClient) do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if v != nil {
		if _, ok := v.(*StringValue); ok {
			responseData, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return resp, err
			}

			responseObject := v.(*StringValue)
			responseObject.Value = string(responseData)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err != nil {
				return resp, err
			}
		}
	}

	return resp, err
}

func NewClient(httpClient *http.Client, baseUrl string, authToken string) *QuayClient {
	quayClient := QuayClient{
		httpClient: httpClient,
		AuthToken:  authToken,
	}

	quayClient.BaseURL, _ = url.Parse(baseUrl)
	return &quayClient
}
