package runtime

import (
	"github.com/azukaar/cosmos-server/src/runtime/types"
	"github.com/azukaar/cosmos-server/src/utils"
)

// InitFromConfig initializes the runtime based on Cosmos config
// This reads the RuntimeType from config and sets up the appropriate backend
func InitFromConfig() error {
	config := utils.GetMainConfig()

	// Default to Docker if not specified
	runtimeType := config.RuntimeType
	if runtimeType == "" {
		runtimeType = "docker"
	}

	var runtimeConfig types.RuntimeConfig

	switch runtimeType {
	case "proxmox":
		runtimeConfig = types.RuntimeConfig{
			Type: types.RuntimeProxmox,
			Proxmox: &types.ProxmoxConfig{
				Host:          config.ProxmoxConfig.Host,
				Node:          config.ProxmoxConfig.Node,
				TokenID:       config.ProxmoxConfig.TokenID,
				TokenSecret:   config.ProxmoxConfig.TokenSecret,
				Storage:       config.ProxmoxConfig.Storage,
				VMIDStart:     config.ProxmoxConfig.VMIDStart,
				VMIDEnd:       config.ProxmoxConfig.VMIDEnd,
				SkipTLSVerify: config.ProxmoxConfig.SkipTLSVerify,
			},
		}
		utils.Log("Initializing Proxmox LXC runtime...")

	default: // "docker" or empty
		runtimeConfig = types.RuntimeConfig{
			Type: types.RuntimeDocker,
			Docker: &types.DockerConfig{
				// Docker config is minimal - uses environment by default
			},
		}
		utils.Log("Initializing Docker runtime...")
	}

	_, err := InitRuntime(runtimeConfig)
	if err != nil {
		utils.Error("Failed to initialize container runtime", err)
		return err
	}

	utils.Log("Container runtime initialized successfully: " + runtimeType)
	return nil
}

// IsDockerMode returns true if Docker runtime is active
func IsDockerMode() bool {
	rt := GetRuntime()
	if rt == nil {
		return true // Default to Docker behavior
	}
	return rt.RuntimeType() == types.RuntimeDocker
}

// IsProxmoxMode returns true if Proxmox runtime is active
func IsProxmoxMode() bool {
	rt := GetRuntime()
	if rt == nil {
		return false
	}
	return rt.RuntimeType() == types.RuntimeProxmox
}

// GetRuntimeTypeFromConfig returns the configured runtime type string
func GetRuntimeTypeFromConfig() string {
	config := utils.GetMainConfig()
	if config.RuntimeType == "" {
		return "docker"
	}
	return config.RuntimeType
}
