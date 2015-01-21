package generator

import (
	"strings"
	"testing"

	"speter.net/go/exp/math/dec/inf"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestGenerateFromMissingDeploymentConfig(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound("deploymentConfig", id)
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "1234")

	if config != nil {
		t.Fatalf("Unexpected deployment config generated: %#v", config)
	}

	if err == nil {
		t.Fatalf("Expected an error")
	}
}

func TestGenerateFromConfigWithoutTagChange(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 1 {
		t.Fatalf("Expected config LatestVersion=1, got %d", config.LatestVersion)
	}
}

func TestGenerateFromZeroConfigWithoutTagChange(t *testing.T) {
	dc := basicDeploymentConfig()
	dc.LatestVersion = 0
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return dc, nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 1 {
		t.Fatalf("Expected config LatestVersion=1, got %d", config.LatestVersion)
	}
}

func TestGenerateFromConfigWithNoDeployment(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy2")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 1 {
		t.Fatalf("Expected config LatestVersion=1, got %d", config.LatestVersion)
	}
}

func TestGenerateFromConfigWithUpdatedImageRef(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				list := okImageRepoList()
				list.Items[0].Tags["tag1"] = "ref2"
				return list, nil
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 2 {
		t.Fatalf("Expected config LatestVersion=2, got %d", config.LatestVersion)
	}

	expected := "registry:8080/repo1:ref2"
	actual := config.Template.ControllerTemplate.Template.Spec.Containers[0].Image
	if expected != actual {
		t.Fatalf("Expected container image %s, got %s", expected, actual)
	}
}

func TestGenerateReportsErrorWhenRepoHasNoImage(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return referenceDeploymentConfig(), nil
			},
			IRFn: func(ctx kapi.Context, name string) (*imageapi.ImageRepository, error) {
				return &emptyImageRepo().Items[0], nil
			},
		},
	}
	_, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")
	if err == nil {
		t.Fatalf("Unexpected non-error")
	}
	if !strings.Contains(err.Error(), "image repository /imageRepo1 does not have a Docker") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateDeploymentConfigWithFrom(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return referenceDeploymentConfig(), nil
			},
			IRFn: func(ctx kapi.Context, name string) (*imageapi.ImageRepository, error) {
				return &internalImageRepo().Items[0], nil
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 2 {
		t.Fatalf("Expected config LatestVersion=2, got %d", config.LatestVersion)
	}

	expected := "internal/namespace/imageRepo1:ref1"
	actual := config.Template.ControllerTemplate.Template.Spec.Containers[0].Image
	if expected != actual {
		t.Fatalf("Expected container image %s, got %s", expected, actual)
	}
}

func okImageRepoList() *imageapi.ImageRepositoryList {
	return &imageapi.ImageRepositoryList{
		Items: []imageapi.ImageRepository{
			{
				ObjectMeta:            kapi.ObjectMeta{Name: "imageRepo1"},
				DockerImageRepository: "registry:8080/repo1",
				Tags: map[string]string{
					"tag1": "ref1",
				},
				Status: imageapi.ImageRepositoryStatus{
					DockerImageRepository: "registry:8080/repo1",
				},
			},
		},
	}
}

func basicPodTemplate() *kapi.PodTemplateSpec {
	return &kapi.PodTemplateSpec{
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:   "container1",
					Image:  "registry:8080/repo1:ref1",
					CPU:    resource.Quantity{Amount: inf.NewDec(0, 3), Format: "DecimalSI"},
					Memory: resource.Quantity{Amount: inf.NewDec(0, 0), Format: "DecimalSI"},
				},
				{
					Name:   "container2",
					Image:  "registry:8080/repo1:ref2",
					CPU:    resource.Quantity{Amount: inf.NewDec(0, 3), Format: "DecimalSI"},
					Memory: resource.Quantity{Amount: inf.NewDec(0, 0), Format: "DecimalSI"},
				},
			},
		},
	}
}

func basicDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta:    kapi.ObjectMeta{Name: "deploy1"},
		LatestVersion: 1,
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					ContainerNames: []string{
						"container1",
					},
					RepositoryName: "registry:8080/repo1",
					Tag:            "tag1",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Template: basicPodTemplate(),
			},
		},
	}
}

func referenceDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta:    kapi.ObjectMeta{Name: "deploy1"},
		LatestVersion: 1,
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					ContainerNames: []string{
						"container1",
					},
					From: kapi.ObjectReference{
						Name: "repo1",
					},
					Tag: "tag1",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Template: basicPodTemplate(),
			},
		},
	}
}

func basicDeployment() *kapi.ReplicationController {
	config := basicDeploymentConfig()
	encodedConfig, _ := deployutil.EncodeDeploymentConfig(config, api.Codec)

	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.LatestDeploymentNameForConfig(config),
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
			},
			Labels: config.Labels,
		},
		Spec: kapi.ReplicationControllerSpec{
			Template: basicPodTemplate(),
		},
	}
}

func internalImageRepo() *imageapi.ImageRepositoryList {
	return &imageapi.ImageRepositoryList{
		Items: []imageapi.ImageRepository{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "imageRepo1"},
				Tags: map[string]string{
					"tag1": "ref1",
				},
				Status: imageapi.ImageRepositoryStatus{
					DockerImageRepository: "internal/namespace/imageRepo1",
				},
			},
		},
	}
}

func emptyImageRepo() *imageapi.ImageRepositoryList {
	return &imageapi.ImageRepositoryList{
		Items: []imageapi.ImageRepository{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "imageRepo1"},
				Tags: map[string]string{
					"tag1": "ref1",
				},
				Status: imageapi.ImageRepositoryStatus{
					DockerImageRepository: "",
				},
			},
		},
	}
}
