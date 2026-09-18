package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// withArgs runs fn with a fresh flag.CommandLine and os.Args, needed since
// NewConfig re-registers its flags on every call.
func withArgs(t *testing.T, args []string, fn func()) {
	t.Helper()

	prevArgs := os.Args
	prevCommandLine := flag.CommandLine
	t.Cleanup(func() {
		os.Args = prevArgs
		flag.CommandLine = prevCommandLine
	})

	os.Args = append([]string{"shortener"}, args...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	fn()
}

// panicOnFatal returns a logger whose Fatal panics instead of os.Exit-ing.
func panicOnFatal() *zap.Logger {
	return zap.New(zapcore.NewNopCore(), zap.WithFatalHook(zapcore.WriteThenPanic))
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0666))
	return path
}

func TestNewConfig_Defaults(t *testing.T) {
	var cfg *Config
	withArgs(t, nil, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "localhost:8080", cfg.ServerAddress)
	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
	assert.Equal(t, "storage.jsonl", cfg.StorageFile)
	assert.Empty(t, cfg.DSN)
	assert.False(t, cfg.EnableHTTPS)
}

func TestNewConfig_FlagsOverrideDefaults(t *testing.T) {
	var cfg *Config
	withArgs(t, []string{"-a", "localhost:9090", "-s"}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "localhost:9090", cfg.ServerAddress)
	assert.True(t, cfg.EnableHTTPS)
}

func TestNewConfig_EnvOverridesFlags(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "localhost:7070")

	var cfg *Config
	withArgs(t, []string{"-a", "localhost:9090"}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "localhost:7070", cfg.ServerAddress)
}

func TestNewConfig_FileFillsInUnsetOptions(t *testing.T) {
	path := writeConfigFile(t, `{
		"server_address": "localhost:9999",
		"base_url": "http://fromfile.example",
		"enable_https": true,
		"audit_file": "/tmp/audit-from-file.txt"
	}`)

	var cfg *Config
	withArgs(t, []string{"-c", path}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "localhost:9999", cfg.ServerAddress)
	assert.Equal(t, "http://fromfile.example", cfg.BaseURL)
	assert.True(t, cfg.EnableHTTPS)
	assert.Equal(t, "/tmp/audit-from-file.txt", cfg.AuditFile)
	// Not set in the file: keeps the flag default.
	assert.Equal(t, "storage.jsonl", cfg.StorageFile)
}

func TestNewConfig_FlagBeatsFile(t *testing.T) {
	path := writeConfigFile(t, `{"server_address": "localhost:9999"}`)

	var cfg *Config
	withArgs(t, []string{"-c", path, "-a", "localhost:1111"}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "localhost:1111", cfg.ServerAddress)
}

func TestNewConfig_EnvBeatsFile(t *testing.T) {
	path := writeConfigFile(t, `{"server_address": "localhost:9999"}`)
	t.Setenv("SERVER_ADDRESS", "localhost:2222")

	var cfg *Config
	withArgs(t, []string{"-c", path}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "localhost:2222", cfg.ServerAddress)
}

func TestNewConfig_ConfigAliasAndEnvVar(t *testing.T) {
	path := writeConfigFile(t, `{"server_address": "localhost:3333"}`)

	var cfgLong *Config
	withArgs(t, []string{"-config", path}, func() {
		cfgLong = NewConfig(zap.NewNop())
	})
	assert.Equal(t, "localhost:3333", cfgLong.ServerAddress)

	t.Setenv("CONFIG", path)
	var cfgEnv *Config
	withArgs(t, nil, func() {
		cfgEnv = NewConfig(zap.NewNop())
	})
	assert.Equal(t, "localhost:3333", cfgEnv.ServerAddress)
}

func TestNewConfig_MissingFileIsFatal(t *testing.T) {
	logger := panicOnFatal()

	assert.Panics(t, func() {
		withArgs(t, []string{"-c", "/no/such/config.json"}, func() {
			NewConfig(logger)
		})
	})
}

func TestNewConfig_InvalidJSONIsFatal(t *testing.T) {
	path := writeConfigFile(t, `{not valid json`)
	logger := panicOnFatal()

	assert.Panics(t, func() {
		withArgs(t, []string{"-c", path}, func() {
			NewConfig(logger)
		})
	})
}
