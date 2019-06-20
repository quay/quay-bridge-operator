package utils

import (
	"testing"
)

func TestRobotAccountName(t *testing.T) {

	cases := []struct {
		name                  string
		organizationName      string
		robotAccountShortname string
		expected              string
	}{
		{
			name:                  "test-robot-account-name",
			organizationName:      "test_org",
			robotAccountShortname: "robot",
			expected:              "test_org+robot",
		},
	}

	for i, c := range cases {

		t.Run(c.name, func(t *testing.T) {

			result := FormatOrganizationRobotAccountName(c.organizationName, c.robotAccountShortname)

			if c.expected != result {
				t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.expected, result)
			}
		})
	}
}
