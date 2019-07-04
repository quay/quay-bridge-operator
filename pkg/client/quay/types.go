package quay

type QuayRole string

const (
	QuayRoleAdmin QuayRole = "admin"
	QuayRoleRead  QuayRole = "read"
	QuayRoleWrite QuayRole = "write"
)

type User struct {
	Username      string         `json:"username"`
	Organizations []Organization `json:"organizations"`
	Email         string         `json:"email"`
}

// Organization
type Organization struct {
}

type OrganizationRequest struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type PrototypesResponse struct {
	Prototypes []Prototype `json:"prototypes"`
}

type RobotAccount struct {
	Description string `json:"description"`
	Created     string `json:"created"`
	// UnstructuredData []byte  `json:"unstructured_metadata"`
	LastAccessed string `json:"last_accessed"`
	Token        string `json:"token"`
	Name         string `json:"name"`
}

type Prototype struct {
	ID       string            `json:"id"`
	Role     string            `json:"role"`
	Delegate PrototypeDelegate `json:"delegate"`
}

type Repository struct {
	TrustEnabled   bool              `json:"trust_enabled"`
	Description    string            `json:"description"`
	CanAdmin       bool              `json:"can_admin"`
	CanWrite       bool              `json:"can_write"`
	IsOrganization bool              `json:"is_organization"`
	IsStarred      bool              `json:"is_starred"`
	IsPublic       bool              `json:"is_public"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Image          string            `json:"image"`
	TagExpirationS int               `json:"tag_expiration_s"`
	Tags           map[string]string `json:"tags"`
	StatusToken    string            `json:"status_token"`
}

type RepositoryRequest struct {
	Namespace   string `json:"namespace"`
	Visibility  string `json:"visibility"`
	Repository  string `json:"repository"`
	Description string `json:"description"`
	Kind        string `json:"repo_kind"`
}

type PrototypeDelegate struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Robot     bool   `json:"is_robot"`
	OrgMember bool   `json:"is_org_member"`
}

// StringValue represents an object containing a single string
type StringValue struct {
	Value string
}

func IsRobotAccountInPrototypeByRole(prototypes []Prototype, robotAccount string, role string) bool {

	for _, prototype := range prototypes {

		if prototype.Role == role && prototype.Delegate.Robot == true && prototype.Delegate.Name == robotAccount {
			return true
		}

	}

	return false

}
