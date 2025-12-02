package runtime

// This file re-exports types from the types package for backward compatibility
// All type definitions are in runtime/types/types.go

import (
	"github.com/azukaar/cosmos-server/src/runtime/types"
)

// Re-export constants
const (
	RuntimeDocker  = types.RuntimeDocker
	RuntimeProxmox = types.RuntimeProxmox

	StateCreated    = types.StateCreated
	StateRunning    = types.StateRunning
	StatePaused     = types.StatePaused
	StateRestarting = types.StateRestarting
	StateExited     = types.StateExited
	StateDead       = types.StateDead

	MountTypeBind   = types.MountTypeBind
	MountTypeVolume = types.MountTypeVolume
	MountTypeTmpfs  = types.MountTypeTmpfs
)

// Re-export types for backward compatibility
type (
	RuntimeType           = types.RuntimeType
	ContainerRuntime      = types.ContainerRuntime
	ContainerConfig       = types.ContainerConfig
	Container             = types.Container
	ContainerState        = types.ContainerState
	ContainerDetails      = types.ContainerDetails
	ContainerStats        = types.ContainerStats
	PortMapping           = types.PortMapping
	VolumeMount           = types.VolumeMount
	MountType             = types.MountType
	VolumeConfig          = types.VolumeConfig
	Volume                = types.Volume
	NetworkConfig         = types.NetworkConfig
	IPAMConfig            = types.IPAMConfig
	IPAMPoolConfig        = types.IPAMPoolConfig
	Network               = types.Network
	NetworkSettings       = types.NetworkSettings
	NetworkEndpoint       = types.NetworkEndpoint
	NetworkConnectOptions = types.NetworkConnectOptions
	PortBinding           = types.PortBinding
	HostConfig            = types.HostConfig
	RestartPolicy         = types.RestartPolicy
	HealthCheckConfig     = types.HealthCheckConfig
	Image                 = types.Image
	LogOptions            = types.LogOptions
	RouteConfig           = types.RouteConfig
	SmartShieldConfig     = types.SmartShieldConfig
	RuntimeConfig         = types.RuntimeConfig
	DockerConfig          = types.DockerConfig
	ProxmoxConfig         = types.ProxmoxConfig
)
