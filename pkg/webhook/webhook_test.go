package webhook

import (
	"encoding/json"
	"testing"

	buildv1 "github.com/openshift/api/build/v1"
	quayv1 "github.com/quay/quay-bridge-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetAdmissionResponseForBuild(t *testing.T) {
	qi := &quayv1.QuayIntegration{
		Spec: quayv1.QuayIntegrationSpec{
			ClusterID:    "testcluster",
			QuayHostname: "https://quay.example.com",
		},
	}

	cases := []struct {
		name         string
		build        *buildv1.Build
		wantAllowed  bool
		wantHasPatch bool
	}{
		{
			name: "nil Output.To allows without panic",
			build: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{Namespace: "bctest", Name: "django-1"},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Output: buildv1.BuildOutput{},
						Strategy: buildv1.BuildStrategy{
							Type: buildv1.SourceBuildStrategyType,
							SourceStrategy: &buildv1.SourceBuildStrategy{
								From: corev1.ObjectReference{
									Kind: "ImageStreamTag",
									Name: "python:3.9-ubi8",
								},
							},
						},
					},
				},
			},
			wantAllowed:  true,
			wantHasPatch: false,
		},
		{
			name: "ImageStreamTag output is rewritten to DockerImage",
			build: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{Namespace: "bctest", Name: "app-1"},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Output: buildv1.BuildOutput{
							To: &corev1.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "app:latest",
							},
						},
						Strategy: buildv1.BuildStrategy{
							Type: buildv1.SourceBuildStrategyType,
							SourceStrategy: &buildv1.SourceBuildStrategy{
								From: corev1.ObjectReference{
									Kind: "ImageStreamTag",
									Name: "python:3.9-ubi8",
								},
							},
						},
					},
				},
			},
			wantAllowed:  true,
			wantHasPatch: true,
		},
		{
			name: "non-ImageStreamTag output is allowed without rewrite",
			build: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{Namespace: "bctest", Name: "app-2"},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Output: buildv1.BuildOutput{
							To: &corev1.ObjectReference{
								Kind: "DockerImage",
								Name: "quay.example.com/org/app:latest",
							},
						},
						Strategy: buildv1.BuildStrategy{
							Type:           buildv1.DockerBuildStrategyType,
							DockerStrategy: &buildv1.DockerBuildStrategy{},
						},
					},
				},
			},
			wantAllowed:  true,
			wantHasPatch: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("getAdmissionResponseForBuild panicked: %v", r)
				}
			}()

			resp := getAdmissionResponseForBuild(c.build, qi)
			if resp == nil {
				t.Fatal("expected non-nil AdmissionResponse")
			}
			if resp.Allowed != c.wantAllowed {
				t.Fatalf("Allowed: got %v, want %v", resp.Allowed, c.wantAllowed)
			}

			hasPatch := len(resp.Patch) > 0
			if hasPatch != c.wantHasPatch {
				t.Fatalf("hasPatch: got %v, want %v", hasPatch, c.wantHasPatch)
			}

			if c.wantHasPatch {
				var ops []map[string]interface{}
				if err := json.Unmarshal(resp.Patch, &ops); err != nil {
					t.Fatalf("unmarshal patch: %v", err)
				}
				foundKind := false
				foundName := false
				for _, op := range ops {
					path, _ := op["path"].(string)
					if path == "/spec/output/to/kind" && op["value"] == "DockerImage" {
						foundKind = true
					}
					if path == "/spec/output/to/name" {
						name, _ := op["value"].(string)
						if name == "quay.example.com/testcluster_bctest/app:latest" {
							foundName = true
						}
					}
				}
				if !foundKind || !foundName {
					t.Fatalf("expected DockerImage rewrite patches, got %#v", ops)
				}
			}
		})
	}
}
