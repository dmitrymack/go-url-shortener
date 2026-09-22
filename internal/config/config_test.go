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

func TestNewConfig_EnvEnableHTTPS_FalseDisablesIt(t *testing.T) {
	t.Setenv("ENABLE_HTTPS", "false")

	var cfg *Config
	withArgs(t, []string{"-s"}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.False(t, cfg.EnableHTTPS, "ENABLE_HTTPS=false must override the -s flag")
}

func TestNewConfig_EnvEnableHTTPS_FalseBeatsFile(t *testing.T) {
	path := writeConfigFile(t, `{"enable_https": true}`)
	t.Setenv("ENABLE_HTTPS", "false")

	var cfg *Config
	withArgs(t, []string{"-c", path}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.False(t, cfg.EnableHTTPS, "ENABLE_HTTPS=false must override the config file value, not just the flag")
}

func TestNewConfig_EnvEnableHTTPS_GarbageFallsBackToFile(t *testing.T) {
	path := writeConfigFile(t, `{"enable_https": true}`)
	t.Setenv("ENABLE_HTTPS", "not-a-bool")

	var cfg *Config
	withArgs(t, []string{"-c", path}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.True(t, cfg.EnableHTTPS, "an unparseable ENABLE_HTTPS must not block the config file value")
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

func TestNewConfig_TrustedSubnet_EmptyByDefault(t *testing.T) {
	var cfg *Config
	withArgs(t, nil, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Empty(t, cfg.TrustedSubnet)
	assert.Nil(t, cfg.TrustedNet, "an empty trusted subnet must stay nil so the stats endpoint denies everyone")
}

func TestNewConfig_TrustedSubnet_FromFlag(t *testing.T) {
	var cfg *Config
	withArgs(t, []string{"-t", "192.168.1.0/24"}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "192.168.1.0/24", cfg.TrustedSubnet)
	require.NotNil(t, cfg.TrustedNet)
	assert.Equal(t, "192.168.1.0/24", cfg.TrustedNet.String())
}

func TestNewConfig_TrustedSubnet_FromFile(t *testing.T) {
	path := writeConfigFile(t, `{"trusted_subnet": "10.0.0.0/8"}`)

	var cfg *Config
	withArgs(t, []string{"-c", path}, func() {
		cfg = NewConfig(zap.NewNop())
	})

	require.NotNil(t, cfg.TrustedNet)
	assert.Equal(t, "10.0.0.0/8", cfg.TrustedNet.String())
}

func TestNewConfig_TrustedSubnet_Priority(t *testing.T) {
	path := writeConfigFile(t, `{"trusted_subnet": "10.0.0.0/8"}`)

	var flagOverFile *Config
	withArgs(t, []string{"-c", path, "-t", "172.16.0.0/12"}, func() {
		flagOverFile = NewConfig(zap.NewNop())
	})
	assert.Equal(t, "172.16.0.0/12", flagOverFile.TrustedSubnet, "a flag beats the config file")

	t.Setenv("TRUSTED_SUBNET", "192.168.0.0/16")
	var envOverFlag *Config
	withArgs(t, []string{"-c", path, "-t", "172.16.0.0/12"}, func() {
		envOverFlag = NewConfig(zap.NewNop())
	})
	assert.Equal(t, "192.168.0.0/16", envOverFlag.TrustedSubnet, "an env var beats both the flag and the config file")
}

func TestNewConfig_TrustedSubnet_InvalidCIDRIsFatal(t *testing.T) {
	logger := panicOnFatal()

	assert.Panics(t, func() {
		withArgs(t, []string{"-t", "not-a-cidr"}, func() {
			NewConfig(logger)
		})
	})
}

func TestNewConfig_GRPCAddress_Default(t *testing.T) {
	var cfg *Config
	withArgs(t, nil, func() {
		cfg = NewConfig(zap.NewNop())
	})

	assert.Equal(t, "localhost:3200", cfg.GRPCAddress)
}

func TestNewConfig_GRPCAddress_Priority(t *testing.T) {
	path := writeConfigFile(t, `{"grpc_address": "localhost:1000"}`)

	var fileOnly *Config
	withArgs(t, []string{"-c", path}, func() {
		fileOnly = NewConfig(zap.NewNop())
	})
	assert.Equal(t, "localhost:1000", fileOnly.GRPCAddress)

	var flagOverFile *Config
	withArgs(t, []string{"-c", path, "-g", "localhost:2000"}, func() {
		flagOverFile = NewConfig(zap.NewNop())
	})
	assert.Equal(t, "localhost:2000", flagOverFile.GRPCAddress, "a flag beats the config file")

	t.Setenv("GRPC_ADDRESS", "localhost:3000")
	var envOverFlag *Config
	withArgs(t, []string{"-c", path, "-g", "localhost:2000"}, func() {
		envOverFlag = NewConfig(zap.NewNop())
	})
	assert.Equal(t, "localhost:3000", envOverFlag.GRPCAddress, "an env var beats both the flag and the config file")
}
