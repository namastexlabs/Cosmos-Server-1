package proxmox

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	runtime "github.com/azukaar/cosmos-server/src/runtime/types"
	"github.com/azukaar/cosmos-server/src/utils"
)

// Config holds Proxmox connection settings
type Config struct {
	Host          string
	Node          string
	TokenID       string
	TokenSecret   string
	Storage       string
	VMIDStart     int
	VMIDEnd       int
	SkipTLSVerify bool
}

// ProxmoxRuntime implements ContainerRuntime for Proxmox LXC
type ProxmoxRuntime struct {
	client      *http.Client
	config      *Config
	apiURL      string
	node        string
	connected   bool
	vmidCounter int
	mutex       sync.RWMutex
	metadata    *MetadataStore
}

// MetadataStore handles container metadata (labels equivalent)
type MetadataStore struct {
	path string
	data map[int]map[string]string // vmid -> labels
	mu   sync.RWMutex
}

// New creates a new Proxmox runtime
func New(config *Config) (*ProxmoxRuntime, error) {
	if config == nil {
		return nil, errors.New("proxmox config is required")
	}

	if config.Host == "" {
		return nil, errors.New("proxmox host is required")
	}

	if config.Node == "" {
		return nil, errors.New("proxmox node is required")
	}

	if config.TokenID == "" || config.TokenSecret == "" {
		return nil, errors.New("proxmox API token is required")
	}

	return &ProxmoxRuntime{
		config:      config,
		node:        config.Node,
		vmidCounter: config.VMIDStart,
		apiURL:      fmt.Sprintf("https://%s/api2/json", config.Host),
		metadata: &MetadataStore{
			path: "/var/lib/cosmos/proxmox-metadata",
			data: make(map[int]map[string]string),
		},
	}, nil
}

// Connect establishes connection to Proxmox API
func (p *ProxmoxRuntime) Connect() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Create HTTP client with optional TLS skip
	tlsConfig := &tls.Config{
		InsecureSkipVerify: p.config.SkipTLSVerify,
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	p.client = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Test connection by getting version
	resp, err := p.apiRequest("GET", "/version", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Proxmox: %w", err)
	}

	if version, ok := resp["version"].(string); ok {
		utils.Log(fmt.Sprintf("Connected to Proxmox VE %s", version))
	}

	// Load metadata
	if err := p.metadata.Load(); err != nil {
		utils.Warn("Failed to load Proxmox metadata: " + err.Error())
	}

	// Update VMID counter
	if err := p.updateVMIDCounter(); err != nil {
		utils.Warn("Failed to update VMID counter: " + err.Error())
	}

	p.connected = true
	return nil
}

// apiRequest makes an authenticated request to the Proxmox API
func (p *ProxmoxRuntime) apiRequest(method, path string, body io.Reader) (map[string]interface{}, error) {
	url := p.apiURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Set API token authentication
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", p.config.TokenID, p.config.TokenSecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if data, ok := result.Data.(map[string]interface{}); ok {
		return data, nil
	}

	return map[string]interface{}{"data": result.Data}, nil
}

// IsConnected returns whether Proxmox is connected
func (p *ProxmoxRuntime) IsConnected() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.connected
}

// Close closes the Proxmox client connection
func (p *ProxmoxRuntime) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Save metadata before closing
	if err := p.metadata.Save(); err != nil {
		utils.Warn("Failed to save Proxmox metadata: " + err.Error())
	}

	p.client = nil
	p.connected = false
	return nil
}

// RuntimeType returns the runtime type
func (p *ProxmoxRuntime) RuntimeType() runtime.RuntimeType {
	return runtime.RuntimeProxmox
}

// Version returns the Proxmox version
func (p *ProxmoxRuntime) Version() string {
	if !p.connected {
		return "unknown"
	}

	resp, err := p.apiRequest("GET", "/version", nil)
	if err != nil {
		return "unknown"
	}

	if version, ok := resp["version"].(string); ok {
		return version
	}
	return "unknown"
}

// getNextVMID allocates the next available VMID
func (p *ProxmoxRuntime) getNextVMID() (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.vmidCounter >= p.config.VMIDEnd {
		return 0, errors.New("VMID range exhausted")
	}

	vmid := p.vmidCounter
	p.vmidCounter++
	return vmid, nil
}

// updateVMIDCounter updates the VMID counter based on existing containers
func (p *ProxmoxRuntime) updateVMIDCounter() error {
	resp, err := p.apiRequest("GET", fmt.Sprintf("/nodes/%s/lxc", p.node), nil)
	if err != nil {
		return err
	}

	maxVMID := p.config.VMIDStart
	if data, ok := resp["data"].([]interface{}); ok {
		for _, item := range data {
			if container, ok := item.(map[string]interface{}); ok {
				if vmid, ok := container["vmid"].(float64); ok {
					if int(vmid) >= maxVMID {
						maxVMID = int(vmid) + 1
					}
				}
			}
		}
	}

	p.vmidCounter = maxVMID
	return nil
}

// Create creates a new LXC container
func (p *ProxmoxRuntime) Create(config runtime.ContainerConfig) (string, error) {
	if !p.connected {
		return "", errors.New("not connected to Proxmox")
	}

	vmid, err := p.getNextVMID()
	if err != nil {
		return "", err
	}

	// Build LXC configuration
	lxcConfig := p.buildLXCConfig(vmid, config)

	// Create the container via API
	configJSON, _ := json.Marshal(lxcConfig)
	_, err = p.apiRequest("POST", fmt.Sprintf("/nodes/%s/lxc", p.node), strings.NewReader(string(configJSON)))
	if err != nil {
		return "", fmt.Errorf("failed to create LXC container: %w", err)
	}

	// Store metadata (labels)
	if len(config.Labels) > 0 {
		p.metadata.Set(vmid, config.Labels)
	}

	// Store name mapping
	p.metadata.SetLabel(vmid, "cosmos-name", config.Name)

	containerID := strconv.Itoa(vmid)
	utils.Log(fmt.Sprintf("Created LXC container %s (VMID: %d)", config.Name, vmid))

	return containerID, nil
}

// buildLXCConfig converts runtime.ContainerConfig to Proxmox LXC config
func (p *ProxmoxRuntime) buildLXCConfig(vmid int, config runtime.ContainerConfig) map[string]interface{} {
	lxc := map[string]interface{}{
		"vmid":         vmid,
		"hostname":     config.Hostname,
		"ostemplate":   config.Image,
		"storage":      p.config.Storage,
		"password":     generateSecurePassword(),
		"unprivileged": !config.Privileged,
		"start":        false,
	}

	if config.Hostname == "" {
		lxc["hostname"] = config.Name
	}

	// Memory (convert bytes to MB)
	if config.Memory > 0 {
		lxc["memory"] = config.Memory / (1024 * 1024)
	} else {
		lxc["memory"] = 512
	}

	// Swap
	if config.MemorySwap > 0 {
		lxc["swap"] = config.MemorySwap / (1024 * 1024)
	} else {
		lxc["swap"] = 512
	}

	// CPUs
	if config.CPUs > 0 {
		lxc["cores"] = int(config.CPUs)
	} else {
		lxc["cores"] = 1
	}

	// Network
	lxc["net0"] = "name=eth0,bridge=vmbr0,ip=dhcp"

	// Mount points
	mpIndex := 0
	for _, vol := range config.Volumes {
		mpKey := fmt.Sprintf("mp%d", mpIndex)
		mpValue := fmt.Sprintf("%s,mp=%s", vol.Source, vol.Target)
		if vol.ReadOnly {
			mpValue += ",ro=1"
		}
		lxc[mpKey] = mpValue
		mpIndex++
	}

	// Root filesystem
	lxc["rootfs"] = fmt.Sprintf("%s:8", p.config.Storage)

	// Features
	lxc["features"] = "nesting=1"

	return lxc
}

// Start starts a container
func (p *ProxmoxRuntime) Start(id string) error {
	vmid, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid container ID: %s", id)
	}

	_, err = p.apiRequest("POST", fmt.Sprintf("/nodes/%s/lxc/%d/status/start", p.node, vmid), nil)
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w", id, err)
	}

	utils.Log(fmt.Sprintf("Started LXC container VMID: %d", vmid))
	return nil
}

// Stop stops a container
func (p *ProxmoxRuntime) Stop(id string) error {
	vmid, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid container ID: %s", id)
	}

	_, err = p.apiRequest("POST", fmt.Sprintf("/nodes/%s/lxc/%d/status/stop", p.node, vmid), nil)
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", id, err)
	}

	utils.Log(fmt.Sprintf("Stopped LXC container VMID: %d", vmid))
	return nil
}

// Restart restarts a container
func (p *ProxmoxRuntime) Restart(id string) error {
	if err := p.Stop(id); err != nil {
		utils.Warn("Stop before restart failed: " + err.Error())
	}

	time.Sleep(2 * time.Second)

	return p.Start(id)
}

// Remove deletes a container
func (p *ProxmoxRuntime) Remove(id string) error {
	vmid, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid container ID: %s", id)
	}

	// Stop first if running
	_ = p.Stop(id)
	time.Sleep(2 * time.Second)

	_, err = p.apiRequest("DELETE", fmt.Sprintf("/nodes/%s/lxc/%d", p.node, vmid), nil)
	if err != nil {
		return fmt.Errorf("failed to delete container %s: %w", id, err)
	}

	// Remove metadata
	p.metadata.Delete(vmid)

	utils.Log(fmt.Sprintf("Removed LXC container VMID: %d", vmid))
	return nil
}

// Recreate recreates a container with new config
func (p *ProxmoxRuntime) Recreate(id string, config runtime.ContainerConfig) (string, error) {
	if err := p.Remove(id); err != nil {
		utils.Warn("Remove during recreate failed: " + err.Error())
	}

	return p.Create(config)
}

// List returns all LXC containers
func (p *ProxmoxRuntime) List() ([]runtime.Container, error) {
	if !p.connected {
		return nil, errors.New("not connected to Proxmox")
	}

	resp, err := p.apiRequest("GET", fmt.Sprintf("/nodes/%s/lxc", p.node), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var containers []runtime.Container
	if data, ok := resp["data"].([]interface{}); ok {
		for _, item := range data {
			if r, ok := item.(map[string]interface{}); ok {
				vmid := int(r["vmid"].(float64))
				container := runtime.Container{
					ID:     strconv.Itoa(vmid),
					Name:   p.metadata.GetLabel(vmid, "cosmos-name"),
					Status: getStatus(r["status"]),
					State:  mapProxmoxState(r["status"]),
					Labels: p.metadata.Get(vmid),
				}

				if container.Name == "" {
					if name, ok := r["name"].(string); ok {
						container.Name = name
					}
				}

				containers = append(containers, container)
			}
		}
	}

	return containers, nil
}

// Inspect returns detailed container information
func (p *ProxmoxRuntime) Inspect(id string) (*runtime.ContainerDetails, error) {
	vmid, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid container ID: %s", id)
	}

	resp, err := p.apiRequest("GET", fmt.Sprintf("/nodes/%s/lxc/%d/config", p.node, vmid), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	hostname := ""
	if h, ok := resp["hostname"].(string); ok {
		hostname = h
	}

	memory := int64(0)
	if m, ok := resp["memory"].(float64); ok {
		memory = int64(m) * 1024 * 1024
	}

	details := &runtime.ContainerDetails{
		Container: runtime.Container{
			ID:     id,
			Name:   p.metadata.GetLabel(vmid, "cosmos-name"),
			Labels: p.metadata.Get(vmid),
		},
		Config: runtime.ContainerConfig{
			Name:     p.metadata.GetLabel(vmid, "cosmos-name"),
			Hostname: hostname,
			Memory:   memory,
		},
	}

	return details, nil
}

// Logs returns container logs
func (p *ProxmoxRuntime) Logs(id string, opts runtime.LogOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("Log streaming not yet implemented for Proxmox LXC\n")), nil
}

// Stats returns container resource usage
func (p *ProxmoxRuntime) Stats(id string) (*runtime.ContainerStats, error) {
	vmid, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid container ID: %s", id)
	}

	resp, err := p.apiRequest("GET", fmt.Sprintf("/nodes/%s/lxc/%d/status/current", p.node, vmid), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats := &runtime.ContainerStats{
		ID:   id,
		Name: p.metadata.GetLabel(vmid, "cosmos-name"),
	}

	if cpu, ok := resp["cpu"].(float64); ok {
		stats.CPUPercent = cpu * 100
	}

	if mem, ok := resp["mem"].(float64); ok {
		stats.MemoryUsage = int64(mem)
	}

	if maxmem, ok := resp["maxmem"].(float64); ok {
		stats.MemoryLimit = int64(maxmem)
		if stats.MemoryLimit > 0 {
			stats.MemoryPercent = float64(stats.MemoryUsage) / float64(stats.MemoryLimit) * 100
		}
	}

	return stats, nil
}

// StatsAll returns stats for all containers
func (p *ProxmoxRuntime) StatsAll() ([]runtime.ContainerStats, error) {
	containers, err := p.List()
	if err != nil {
		return nil, err
	}

	var allStats []runtime.ContainerStats
	for _, c := range containers {
		stats, err := p.Stats(c.ID)
		if err != nil {
			continue
		}
		allStats = append(allStats, *stats)
	}

	return allStats, nil
}

// Helper functions

func mapProxmoxState(status interface{}) runtime.ContainerState {
	s, ok := status.(string)
	if !ok {
		return runtime.StateDead
	}

	switch s {
	case "running":
		return runtime.StateRunning
	case "stopped":
		return runtime.StateExited
	case "paused":
		return runtime.StatePaused
	default:
		return runtime.StateDead
	}
}

func getStatus(status interface{}) string {
	if s, ok := status.(string); ok {
		return s
	}
	return "unknown"
}

func generateSecurePassword() string {
	// Generate a random password for container creation
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	password := make([]byte, 16)
	for i := range password {
		password[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return string(password)
}
