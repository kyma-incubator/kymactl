package installation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	dockerConfigFile "github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	imageTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/kyma-project/cli/pkg/installation/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func Test_SplitDockerDomain(t *testing.T) {
	test1 := "localhost:5000/test/testImage:1"
	d1, r1 := splitDockerDomain(test1)
	require.Equal(t, d1, "localhost:5000")
	require.Equal(t, r1, "test/testImage:1")

	test2 := "eu.gcr.io/test/testImage"
	d2, r2 := splitDockerDomain(test2)
	require.Equal(t, d2, "eu.gcr.io")
	require.Equal(t, r2, "test/testImage")

	test3 := "testImage"
	d3, r3 := splitDockerDomain(test3)
	require.Equal(t, d3, "index.docker.io")
	require.Equal(t, r3, "testImage")
}

var exampleAuth = types.AuthConfig{
	Username:      "user",
	Password:      "pass",
	Auth:          "",
	ServerAddress: "1.2.3.4",
	Email:         "foo@bar.com",
	IdentityToken: "identityFoo",
	RegistryToken: "registryFoo",
}

var expectedAuth = types.AuthConfig{
	Username:      "user",
	Password:      "pass",
	IdentityToken: "identityFoo",
	RegistryToken: "registryFoo",
}

func genConfigFile(user, pwd string) *dockerConfigFile.ConfigFile {
	configFile := dockerConfigFile.New("tmpConfig")

	authStr := exampleAuth.Username + ":" + exampleAuth.Password

	msg := []byte(authStr)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	exampleAuth.Auth = string(encoded)
	configFile.AuthConfigs["example.com"] = exampleAuth
	return configFile
}

func Test_Resolve_happy_path(t *testing.T) {

	tmpHome, err := ioutil.TempDir("/tmp", "config-test")
	assert.NilError(t, err)
	defer os.RemoveAll(tmpHome)

	configFile := genConfigFile("user", "pass")
	b, err := json.Marshal(configFile)
	assert.NilError(t, err)
	tmpFile := fmt.Sprintf("%s/config.json", tmpHome)
	ioutil.WriteFile(tmpFile, b, 0644)

	os.Setenv("DOCKER_CONFIG", tmpHome)

	dockerCFG, err := resolve("example.com")
	assert.NilError(t, err)
	assert.Equal(t, dockerCFG.Username, "user")
	assert.Equal(t, dockerCFG.Password, "pass")
}

func Test_Resolve_no_file(t *testing.T) {
	os.Setenv("DOCKER_CONFIG", "file-not-exist")
	_, err := resolve("example.com/foo")
	assert.ErrorContains(t, err, "file not found")
}

func Test_BuildKymaInstaller(t *testing.T) {
	imageName := "kyma-project-foo"
	fooLocalSrcPath := "foo"

	// mocks
	mockDocker := &mocks.DockerService{}
	installer := Installation{
		Docker: mockDocker,
	}
	stringReader := strings.NewReader("foo")
	fooReadCloser := ioutil.NopCloser(stringReader)

	fooArchiveTarOptions := &archive.TarOptions{}

	mockDocker.On("ArchiveDirectory", fooLocalSrcPath, fooArchiveTarOptions).Return(fooReadCloser, nil)
	// as context.deadline can have different clocks assume mock.anything here
	mockDocker.On("NegotiateDockerAPIVersion", mock.Anything).Return(nil)
	fooArgs := make(map[string]*string)
	fooImageBuildOptions := imageTypes.ImageBuildOptions{
		Tags:           []string{strings.TrimSpace(string(imageName))},
		SuppressOutput: true,
		Remove:         true,
		Dockerfile:     path.Join("tools", "kyma-installer", "kyma.Dockerfile"),
		BuildArgs:      fooArgs,
	}
	fooImageBuildRes := imageTypes.ImageBuildResponse{
		Body:   fooReadCloser,
		OSType: "fooUnix",
	}
	mockDocker.On("DockerImageBuild", mock.Anything, fooReadCloser, fooImageBuildOptions).Return(fooImageBuildRes, nil)

	// test the function
	err := installer.BuildKymaInstaller(fooLocalSrcPath, imageName)
	assert.NilError(t, err)
}

func Test_PushKymaInstaller(t *testing.T) {
	tmpHome, err := ioutil.TempDir("/tmp", "config-pus-kyma-test")
	assert.NilError(t, err)
	defer os.RemoveAll(tmpHome)

	configFile := genConfigFile("user", "pass")
	b, err := json.Marshal(configFile)
	assert.NilError(t, err)
	tmpFile := fmt.Sprintf("%s/config.json", tmpHome)
	ioutil.WriteFile(tmpFile, b, 0644)

	os.Setenv("DOCKER_CONFIG", tmpHome)
	image := "example.com/foo"

	// mocks
	mockDocker := &mocks.DockerService{}
	installer := Installation{
		Docker: mockDocker,
	}
	// as context.deadline can have different clocks assume mock.anything here
	mockDocker.On("NegotiateDockerAPIVersion", mock.Anything).Return(nil)

	encodedJSON, _ := json.Marshal(expectedAuth)
	fooAuthStr := base64.URLEncoding.EncodeToString(encodedJSON)
	imagePushOptions := imageTypes.ImagePushOptions{RegistryAuth: fooAuthStr}
	stringReader := strings.NewReader("foo")
	fooReadCloser := ioutil.NopCloser(stringReader)
	t.Logf("auhtstr: %v", fooAuthStr)

	mockDocker.On("ImagePush", mock.Anything, image, imagePushOptions).Return(fooReadCloser, nil)

	err = installer.PushKymaInstaller(image)
	assert.NilError(t, err)

}
