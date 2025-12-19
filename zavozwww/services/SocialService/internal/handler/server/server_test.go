package config

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustLoad_Success(t *testing.T) {
	content := []byte(`
server:
  host: "127.0.0.1"
  port: "9090"
services:
  user_service_url: "http://test-user:8081"
  movie_service_url: "http://test-movie:8082"
`)
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(content)
	require.NoError(t, err)
	tmpfile.Close()

	os.Setenv("APP_CONFIG_PATH", tmpfile.Name())
	defer os.Unsetenv("APP_CONFIG_PATH")

	cfg := MustLoad()

	assert.NotNil(t, cfg)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, "http://test-user:8081", cfg.Services.UserServiceURL)
	assert.Equal(t, "http://test-movie:8082", cfg.Services.MovieServiceURL)
}

func TestMustLoad_Defaults(t *testing.T) {
	content := []byte(`
server: {}
services: {}
`)
	tmpfile, err := os.CreateTemp("", "config-defaults-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(content)
	require.NoError(t, err)
	tmpfile.Close()

	os.Setenv("APP_CONFIG_PATH", tmpfile.Name())
	defer os.Unsetenv("APP_CONFIG_PATH")

	cfg := MustLoad()

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "http://localhost:8081", cfg.Services.UserServiceURL)
	assert.Equal(t, "http://localhost:8082", cfg.Services.MovieServiceURL)
}

func TestMustLoad_FileNotFound(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		os.Setenv("APP_CONFIG_PATH", "non_existent_file.yaml")
		MustLoad()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMustLoad_FileNotFound")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestMustLoad_InvalidConfig(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		content := []byte(`invalid_yaml: [`)
		tmpfile, _ := os.CreateTemp("", "config-invalid-*.yaml")
		defer os.Remove(tmpfile.Name())
		tmpfile.Write(content)
		tmpfile.Close()

		os.Setenv("APP_CONFIG_PATH", tmpfile.Name())
		MustLoad()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMustLoad_InvalidConfig")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}
