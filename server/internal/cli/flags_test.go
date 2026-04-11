package cli

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestFlagOrEnv_FlagSet(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("server", "", "server URL")
	cmd.Flags().Set("server", "https://flag.example.com")

	got := FlagOrEnv(cmd, "server", "SERVER_URL", "default")
	if got != "https://flag.example.com" {
		t.Errorf("got %q, want %q", got, "https://flag.example.com")
	}
}

func TestFlagOrEnv_EnvFallback(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("server", "", "server URL")
	// flag not set

	os.Setenv("TEST_SERVER_URL", "https://env.example.com")
	defer os.Unsetenv("TEST_SERVER_URL")

	got := FlagOrEnv(cmd, "server", "TEST_SERVER_URL", "default")
	if got != "https://env.example.com" {
		t.Errorf("got %q, want %q", got, "https://env.example.com")
	}
}

func TestFlagOrEnv_DefaultFallback(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("server", "", "server URL")
	// flag not set, env not set

	os.Unsetenv("TEST_FLAG_OR_ENV_DEFAULT")

	got := FlagOrEnv(cmd, "server", "TEST_FLAG_OR_ENV_DEFAULT", "fallback-value")
	if got != "fallback-value" {
		t.Errorf("got %q, want %q", got, "fallback-value")
	}
}

func TestFlagOrEnv_FlagTakesPriorityOverEnv(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("token", "", "auth token")
	cmd.Flags().Set("token", "flag-token")

	os.Setenv("TEST_TOKEN", "env-token")
	defer os.Unsetenv("TEST_TOKEN")

	got := FlagOrEnv(cmd, "token", "TEST_TOKEN", "default")
	if got != "flag-token" {
		t.Errorf("got %q, want %q (flag should take priority)", got, "flag-token")
	}
}

func TestFlagOrEnv_EmptyEnvIgnored(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("name", "", "name")

	os.Setenv("TEST_NAME_OR_ENV", "   ")
	defer os.Unsetenv("TEST_NAME_OR_ENV")

	got := FlagOrEnv(cmd, "name", "TEST_NAME_OR_ENV", "default-name")
	if got != "default-name" {
		t.Errorf("got %q, want %q (whitespace-only env should be ignored)", got, "default-name")
	}
}
