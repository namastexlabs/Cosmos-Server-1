package types

import "io"

// RuntimeType identifies the container runtime backend
type RuntimeType string

const (
	RuntimeDocker  RuntimeType = "docker"
	RuntimeProxmox RuntimeType = "proxmox"
)

// ContainerConfig defines container creation parameters (runtime-agnostic)
type ContainerConfig struct {
	Name        string
	Image       string // Docker image or LXC template
	Hostname    string
	Domainname  string
	User        string
	WorkingDir  string
	Entrypoint  []string
	Command     []string
	Environment map[string]string
	Labels      map[string]string
	Ports       []PortMapping
	Volumes     []VolumeMount
	Networks    []string

	// Resource limits
	Memory     int64   // bytes
	MemorySwap int64   // bytes
	CPUs       float64
	CPUShares  int64

	// Behavior
	RestartPolicy RestartPolicy
	Privileged    bool
	TTY           bool
	StdinOpen     bool

	// Health check
	HealthCheck *HealthCheckConfig

	// DNS
	DNS        []string
	DNSSearch  []string
	ExtraHosts []string

	// Security
	CapAdd      []string
	CapDrop     []string
	SecurityOpt []string

	// Cosmos-specific
	Routes      []RouteConfig
	PostInstall []string
}

// Container represents a running or stopped container
type Container struct {
	ID       string
	Name     string
	Image    string
	State    ContainerState
	Status   string
	Created  int64
	Labels   map[string]string
	Ports    []PortMapping
	Networks []string
}

// ContainerState represents container lifecycle state
type ContainerState string

const (
	StateCreated    ContainerState = "created"
	StateRunning    ContainerState = "running"
	StatePaused     ContainerState = "paused"
	StateRestarting ContainerState = "restarting"
	StateExited     ContainerState = "exited"
	StateDead       ContainerState = "dead"
)

// ContainerDetails provides full container inspection data
type ContainerDetails struct {
	Container
	Config          ContainerConfig
	NetworkSettings NetworkSettings
	Mounts          []VolumeMount
	HostConfig      HostConfig
}

// ContainerStats holds resource usage statistics
type ContainerStats struct {
	ID            string
	Name          string
	CPUPercent    float64
	MemoryUsage   int64
	MemoryLimit   int64
	MemoryPercent float64
	NetworkRx     int64
	NetworkTx     int64
	BlockRead     int64
	BlockWrite    int64
}

// PortMapping defines port exposure
type PortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string // tcp, udp
}

// VolumeMount defines storage mounting
type VolumeMount struct {
	Type        MountType
	Source      string
	Target      string
	ReadOnly    bool
	Consistency string
}

// MountType identifies volume mount types
type MountType string

const (
	MountTypeBind   MountType = "bind"
	MountTypeVolume MountType = "volume"
	MountTypeTmpfs  MountType = "tmpfs"
)

// VolumeConfig defines named volume creation
type VolumeConfig struct {
	Name   string
	Driver string
	Labels map[string]string
}

// Volume represents a storage volume
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	Labels     map[string]string
	CreatedAt  string
}

// NetworkConfig defines network creation
type NetworkConfig struct {
	Name       string
	Driver     string
	Internal   bool
	EnableIPv6 bool
	IPAM       *IPAMConfig
	Labels     map[string]string
}

// IPAMConfig defines IP address management
type IPAMConfig struct {
	Driver string
	Config []IPAMPoolConfig
}

// IPAMPoolConfig defines IP pool configuration
type IPAMPoolConfig struct {
	Subnet  string
	Gateway string
	IPRange string
}

// Network represents a container network
type Network struct {
	ID       string
	Name     string
	Driver   string
	Scope    string
	Internal bool
	Labels   map[string]string
	IPAM     *IPAMConfig
}

// NetworkSettings holds container network configuration
type NetworkSettings struct {
	Networks   map[string]NetworkEndpoint
	IPAddress  string
	Gateway    string
	MacAddress string
	Ports      map[string][]PortBinding
}

// NetworkEndpoint represents container connection to a network
type NetworkEndpoint struct {
	NetworkID  string
	IPAddress  string
	Gateway    string
	MacAddress string
	Aliases    []string
}

// NetworkConnectOptions for connecting container to network
type NetworkConnectOptions struct {
	Aliases     []string
	IPAddress   string
	IPv6Address string
}

// PortBinding represents host port binding
type PortBinding struct {
	HostIP   string
	HostPort string
}

// HostConfig holds host-specific container settings
type HostConfig struct {
	NetworkMode   string
	RestartPolicy RestartPolicy
	Privileged    bool
	Binds         []string
	PortBindings  map[string][]PortBinding
	DNS           []string
	DNSSearch     []string
	ExtraHosts    []string
	CapAdd        []string
	CapDrop       []string
}

// RestartPolicy defines container restart behavior
type RestartPolicy struct {
	Name              string // always, unless-stopped, on-failure, no
	MaximumRetryCount int
}

// HealthCheckConfig defines container health monitoring
type HealthCheckConfig struct {
	Test        []string
	Interval    int64 // nanoseconds
	Timeout     int64
	Retries     int
	StartPeriod int64
}

// Image represents a container image or LXC template
type Image struct {
	ID      string
	Name    string
	Tags    []string
	Size    int64
	Created int64
}

// LogOptions for retrieving container logs
type LogOptions struct {
	Follow     bool
	Timestamps bool
	Tail       string
	Since      string
	Until      string
}

// RouteConfig for Cosmos reverse proxy routes
type RouteConfig struct {
	Name          string
	Description   string
	UseHost       bool
	Host          string
	UsePathPrefix bool
	PathPrefix    string
	Target        string
	Mode          string
	SmartShield   SmartShieldConfig
}

// SmartShieldConfig for route protection
type SmartShieldConfig struct {
	Enabled bool
}

// ContainerRuntime defines the interface for container orchestration backends
type ContainerRuntime interface {
	// Connection
	Connect() error
	IsConnected() bool
	Close() error

	// Container Lifecycle
	Create(config ContainerConfig) (string, error)
	Start(id string) error
	Stop(id string) error
	Restart(id string) error
	Remove(id string) error
	Recreate(id string, config ContainerConfig) (string, error)

	// Container Info
	List() ([]Container, error)
	Inspect(id string) (*ContainerDetails, error)
	Logs(id string, opts LogOptions) (io.ReadCloser, error)
	Stats(id string) (*ContainerStats, error)
	StatsAll() ([]ContainerStats, error)

	// Network Operations
	CreateNetwork(config NetworkConfig) (string, error)
	RemoveNetwork(id string) error
	ListNetworks() ([]Network, error)
	ConnectToNetwork(containerID, networkID string, opts NetworkConnectOptions) error
	DisconnectFromNetwork(containerID, networkID string) error

	// Volume/Storage Operations
	CreateVolume(config VolumeConfig) (string, error)
	RemoveVolume(id string) error
	ListVolumes() ([]Volume, error)

	// Image/Template Operations
	PullImage(ref string) (io.ReadCloser, error)
	ListImages() ([]Image, error)
	RemoveImage(id string) error

	// Runtime Info
	RuntimeType() RuntimeType
	Version() string
}

// RuntimeConfig holds runtime-specific configuration
type RuntimeConfig struct {
	Type    RuntimeType
	Docker  *DockerConfig
	Proxmox *ProxmoxConfig
}

// DockerConfig for Docker runtime
type DockerConfig struct {
	Host      string // unix:///var/run/docker.sock or tcp://host:port
	TLSVerify bool
	CertPath  string
}

// ProxmoxConfig for Proxmox LXC runtime
type ProxmoxConfig struct {
	Host          string // proxmox.local:8006
	Node          string // pve
	TokenID       string // user@realm!tokenid
	TokenSecret   string
	Storage       string // local-lvm
	VMIDStart     int    // Starting VMID for containers
	VMIDEnd       int    // Ending VMID range
	SkipTLSVerify bool
}
