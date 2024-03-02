package quay_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/quay/quay-bridge-operator/pkg/client/quay"
	mock_quay "github.com/quay/quay-bridge-operator/pkg/client/quay/mocks"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

func TestGetUser(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		body           string
		wantUser       quay.User
		wantErr        string
	}{
		{
			name:           "happy path - returns user info",
			respStatusCode: 200,
			body:           `{"username": "test_user", "email": "test@buynlarge.com"}`,
			wantUser: quay.User{
				Username: "test_user",
				Email:    "test@buynlarge.com",
			},
		},
		{
			name:    "network error during request - http error",
			wantErr: "http error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			u, resp, err := cli.GetUser()

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.Equal(t, tt.wantUser, u)
			assert.NotNil(t, resp)
		})
	}
}

func TestGetOrganizationByName(t *testing.T) {
	tests := []struct {
		name             string
		respStatusCode   int
		body             string
		wantOrganization quay.Organization
		wantErr          string
	}{
		{
			name:           "GET organization without error",
			respStatusCode: 200,
			body:           `{"name": "buynlarge"}`,
		},
		{
			name:    "GET organization with error",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			o, resp, err := cli.GetOrganizationByName("buynlarge")

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.NotNil(t, o)
		})
	}
}

func TestCreateOrganizationByName(t *testing.T) {
	tests := []struct {
		name             string
		respStatusCode   int
		orgName          string
		body             string
		wantOrganization quay.Organization
		wantErr          string
	}{
		{
			name:           "Create organization without error",
			orgName:        "org1",
			respStatusCode: 200,
			body:           `{"name": "org1", "email":"org1@redhat.com"}`,
		},
		{
			name:    "Create organization with error",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			o, resp, err := cli.CreateOrganization(tt.name)

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.NotNil(t, o)
		})
	}
}

func TestCreateOrganizationRobotAccount(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		orgName        string
		robotName      string
		body           string
		wantRobot      quay.RobotAccount
		wantErr        string
	}{
		{
			name:           "Create a RobotAccount without error",
			orgName:        "org1",
			robotName:      "robot",
			respStatusCode: 200,
			body:           `{"name": "org1+robot", "description":"org1s robot account", "token":"abc123"}`,
			wantRobot: quay.RobotAccount{
				Name:        "org1+robot",
				Description: "org1s robot account",
				Token:       "abc123",
			},
		},
		{
			name:    "Create a RobotAccount with error",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			r, resp, err := cli.CreateOrganizationRobotAccount(tt.orgName, tt.robotName)

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.NotNil(t, r)
			assert.Equal(t, tt.wantRobot, r)
		})
	}
}

func TestGetOrganizationRobotAccount(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		orgName        string
		robotName      string
		body           string
		wantRobot      quay.RobotAccount
		wantErr        string
	}{
		{
			name:           "GET a RobotAccount without error",
			orgName:        "org1",
			robotName:      "robot",
			respStatusCode: 200,
			body:           `{"name": "org1+robot", "description":"org1s robot account", "token":"abc123"}`,
			wantRobot: quay.RobotAccount{
				Name:        "org1+robot",
				Description: "org1s robot account",
				Token:       "abc123",
			},
		},
		{
			name:    "GET a RobotAccount with error",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			r, resp, err := cli.GetOrganizationRobotAccount(tt.orgName, tt.robotName)

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.NotNil(t, r)
			assert.Equal(t, tt.wantRobot, r)
		})
	}
}

func TestGetPrototypesByOrganization(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		orgName        string
		body           string
		wantPrototypes quay.PrototypesResponse
		wantErr        string
	}{
		{
			name:           "GET a PrototypeByOrganization without error",
			respStatusCode: 200,
			orgName:        "org1",
			body: `
			{
				"prototypes": [
				  {
					"id": "1",
					"role": "read",
					"delegate": {
					  "kind": "user",
					  "name": "robot",
					  "is_robot": true,
					  "is_org_member": true
					}
				  }
				]
			  }
			`,

			wantPrototypes: quay.PrototypesResponse{Prototypes: []quay.Prototype{
				{
					ID:   "1",
					Role: "read",
					Delegate: quay.PrototypeDelegate{
						Kind:      "user",
						Name:      "robot",
						Robot:     true,
						OrgMember: true,
					},
				},
			}},
		},
		{
			name:    "GET a PrototypeByOrganization with error",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			p, resp, err := cli.GetPrototypesByOrganization(tt.orgName)

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.Equal(t, tt.wantPrototypes, p)
		})
	}
}

func TestDeleteOrganization(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		orgName        string
		body           string
		wantErr        string
	}{
		{
			name:           "DELETE an Organization without error",
			respStatusCode: 204,
			orgName:        "org1",
		},
		{
			name:    "DELETE an Organization with error",
			orgName: "org1",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			resp, err := cli.DeleteOrganization(tt.orgName)

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
		})
	}
}

func TestCreateRobotPermissionForOrganization(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		orgName        string
		robotName      string
		body           string
		wantErr        string
		wantPrototype  quay.Prototype
	}{
		{
			name:           "Assign Robot permissions without error",
			respStatusCode: 200,
			orgName:        "org1",
			robotName:      "robot",
			body: `
			{
				"id": "1",
				"role": "write",
				"delegate": {
				  "kind": "user",
				  "name": "robot",
				  "is_robot": true,
				  "is_org_member": true
				}
			}
			`,
			wantPrototype: quay.Prototype{
				ID:   "1",
				Role: "write",
				Delegate: quay.PrototypeDelegate{
					Kind:      "user",
					Name:      "robot",
					Robot:     true,
					OrgMember: true,
				},
			},
		},
		{
			name:    "Assign Robot permissions with error",
			orgName: "org1",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			p, resp, err := cli.CreateRobotPermissionForOrganization(tt.orgName, tt.robotName, "write")

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.Equal(t, tt.wantPrototype, p)
		})
	}
}

func TestGetRepository(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		orgName        string
		repoName       string
		body           string
		wantErr        string
		wantRepository quay.Repository
	}{
		{
			name:           "GET repository without error",
			respStatusCode: 200,
			orgName:        "org1",
			repoName:       "repo1",
			body:           `{"namespace": "org1", "name": "repo1", "kind": "image"}`,
			wantRepository: quay.Repository{
				Description: "",
				IsPublic:    false,
				Name:        "repo1",
				Namespace:   "org1",
			},
		},
		{
			name:    "GET repository with error",
			orgName: "org1",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			r, resp, err := cli.GetRepository(tt.orgName, tt.repoName)

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.Equal(t, tt.wantRepository, r)
		})
	}
}

func TestCreateRepository(t *testing.T) {
	tests := []struct {
		name           string
		respStatusCode int
		orgName        string
		repoName       string
		body           string
		wantErr        string
		wantRepository quay.RepositoryRequest
	}{
		{
			name:           "Create repository without error",
			respStatusCode: 201,
			orgName:        "org1",
			repoName:       "repo1",
			body:           `{"namespace": "org1", "visibility": "private", "repository": "repo1", "description": "", "repo_kind": "image"}`,
			wantRepository: quay.RepositoryRequest{
				Namespace:   "org1",
				Visibility:  "private",
				Repository:  "repo1",
				Description: "",
				Kind:        "image",
			},
		},
		{
			name:    "Create repository with error",
			orgName: "org1",
			body:    `{"name", "buynlarge"}`,
			wantErr: "{invalid character ',' after object key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			mockResp := &http.Response{
				StatusCode: tt.respStatusCode,
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.body))),
			}

			var e error
			if tt.wantErr == "" {
				e = nil
			} else {
				e = fmt.Errorf(tt.wantErr)
			}

			mockClient.EXPECT().Do(gomock.Any()).Return(mockResp, e)

			r, resp, err := cli.CreateRepository(tt.orgName, tt.repoName)

			if (err.Error == nil && tt.wantErr != "") || (err.Error != nil && err.Error.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
			assert.Equal(t, tt.respStatusCode, resp.StatusCode)
			assert.Equal(t, tt.wantRepository, r)
		})
	}
}

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		endpoint    string
		body        any
		wantHeaders http.Header
		wantMethod  string
		wantErr     string
	}{
		{
			name:     "happy path - returns new request",
			method:   "GET",
			endpoint: "/my/endpoint",
			body:     `{"user":"buynlarge"}`,
		},
		{
			name:     "invalid body payload - catch json decode error",
			method:   "GET",
			endpoint: "/my/endpoint",
			body:     make(chan int),
			wantErr:  "json: unsupported type: chan int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock_quay.NewMockHttpClient(ctrl)
			cli := quay.NewClient(mockClient, "localhost", "my-secret-token")

			resp, err := cli.NewRequest(tt.method, tt.endpoint, tt.body)
			if (err == nil && tt.wantErr != "") || (err != nil && err.Error() != tt.wantErr) {
				t.Errorf("wanted err to be %v, but got %v", tt.wantErr, err)
			}

			if tt.wantErr != "" {
				assert.Equal(t, err.Error(), tt.wantErr)
				return
			}

			assert.NotNil(t, resp)
		})
	}
}
