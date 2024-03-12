package quay

import (
	"testing"
)

func TestIsRobotAccountInPrototypeByRole(t *testing.T) {
	cases := []struct {
		name         string
		prototypes   []Prototype
		robotAccount string
		role         string
		expected     bool
	}{
		{
			name: "test-valid-robot-account-in-prototype-by-role",
			prototypes: []Prototype{
				{
					Role: "read",
					Delegate: PrototypeDelegate{
						Kind:  "user",
						Robot: true,
						Name:  "robot",
					},
				},
			},
			robotAccount: "robot",
			role:         "read",
			expected:     true,
		},
		{
			name: "test-invalid-robot-account-in-prototype-by-role",
			prototypes: []Prototype{
				{
					Role: "read",
					Delegate: PrototypeDelegate{
						Kind:  "user",
						Robot: true,
						Name:  "robot",
					},
				},
			},
			robotAccount: "robot",
			role:         "write",
			expected:     false,
		},
		{
			name: "test-non-robot-account-in-prototype-by-role",
			prototypes: []Prototype{
				{
					Role: "read",
					Delegate: PrototypeDelegate{
						Kind:  "user",
						Robot: false,
						Name:  "robot",
					},
				},
			},
			robotAccount: "robot",
			role:         "read",
			expected:     false,
		},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := IsRobotAccountInPrototypeByRole(c.prototypes, c.robotAccount, c.role)
			if c.expected != result {
				t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.expected, result)
			}
		})
	}
}
