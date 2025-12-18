# Quay API Client

## Location
`pkg/client/quay/`

## Client Initialization

```go
quayClient := qclient.NewClient(&http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    },
}, quayHostname, authToken)
```

## Available Operations

### Organizations
- `GetOrganizationByName(name)` - Check if org exists
- `CreateOrganization(name)` - Create new org
- `DeleteOrganization(name)` - Delete org and all contents

### Robot Accounts
- `GetOrganizationRobotAccount(org, name)` - Get robot account
- `CreateOrganizationRobotAccount(org, name)` - Create robot account

### Repositories
- `GetRepository(org, name)` - Check if repo exists
- `CreateRepository(org, name)` - Create new repo

### Prototypes (Default Permissions)
- `GetPrototypesByOrganization(org)` - List permission prototypes
- `CreateRobotPermissionForOrganization(org, robot, role)` - Add prototype

## Response Handling

All methods return three values:
1. Response object (typed)
2. `*http.Response` for status code checks
3. Error object with `.Error` field

Example pattern:
```go
org, response, err := quayClient.GetOrganizationByName(name)
if err.Error != nil {
    // Handle error
}
if response.StatusCode == 404 {
    // Organization doesn't exist
}
```

## Roles
- `QuayRoleRead` - Pull access
- `QuayRoleWrite` - Push access

## Testing
Mocks available in `pkg/client/quay/mocks/client_mock.go`
