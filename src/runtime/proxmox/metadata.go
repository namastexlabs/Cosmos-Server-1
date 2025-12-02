package proxmox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// MetadataStore manages container labels and metadata
// Since Proxmox LXC doesn't have Docker-style labels,
// we store metadata in a local JSON file

// Load reads metadata from disk
func (m *MetadataStore) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath := filepath.Join(m.path, "containers.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(m.path, 0755); err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		m.data = make(map[int]map[string]string)
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.data)
}

// Save writes metadata to disk
func (m *MetadataStore) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath := filepath.Join(m.path, "containers.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(m.path, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// Get returns all labels for a container
func (m *MetadataStore) Get(vmid int) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if labels, ok := m.data[vmid]; ok {
		// Return a copy to prevent race conditions
		copy := make(map[string]string)
		for k, v := range labels {
			copy[k] = v
		}
		return copy
	}
	return nil
}

// Set sets all labels for a container
func (m *MetadataStore) Set(vmid int, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data == nil {
		m.data = make(map[int]map[string]string)
	}

	m.data[vmid] = labels

	// Auto-save after modification
	go m.saveAsync()
}

// GetLabel returns a specific label
func (m *MetadataStore) GetLabel(vmid int, key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if labels, ok := m.data[vmid]; ok {
		return labels[key]
	}
	return ""
}

// SetLabel sets a specific label
func (m *MetadataStore) SetLabel(vmid int, key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data == nil {
		m.data = make(map[int]map[string]string)
	}

	if m.data[vmid] == nil {
		m.data[vmid] = make(map[string]string)
	}

	m.data[vmid][key] = value

	// Auto-save after modification
	go m.saveAsync()
}

// Delete removes all metadata for a container
func (m *MetadataStore) Delete(vmid int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, vmid)

	// Auto-save after modification
	go m.saveAsync()
}

// HasLabel checks if a label exists
func (m *MetadataStore) HasLabel(vmid int, key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if labels, ok := m.data[vmid]; ok {
		_, exists := labels[key]
		return exists
	}
	return false
}

// FindByLabel finds containers with a specific label value
func (m *MetadataStore) FindByLabel(key, value string) []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []int
	for vmid, labels := range m.data {
		if labels[key] == value {
			results = append(results, vmid)
		}
	}
	return results
}

// FindByName finds a container by cosmos-name label
func (m *MetadataStore) FindByName(name string) int {
	results := m.FindByLabel("cosmos-name", name)
	if len(results) > 0 {
		return results[0]
	}
	return 0
}

// saveAsync saves metadata asynchronously
func (m *MetadataStore) saveAsync() {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath := filepath.Join(m.path, "containers.json")

	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(filePath, data, 0644)
}

// VMIDMapping stores mapping between container names and VMIDs
type VMIDMapping struct {
	mu      sync.RWMutex
	nameToID map[string]int
	idToName map[int]string
}

// NewVMIDMapping creates a new mapping store
func NewVMIDMapping() *VMIDMapping {
	return &VMIDMapping{
		nameToID: make(map[string]int),
		idToName: make(map[int]string),
	}
}

// Set adds a name-VMID mapping
func (v *VMIDMapping) Set(name string, vmid int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.nameToID[name] = vmid
	v.idToName[vmid] = name
}

// GetByName returns VMID for a container name
func (v *VMIDMapping) GetByName(name string) (int, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	vmid, ok := v.nameToID[name]
	return vmid, ok
}

// GetByID returns name for a VMID
func (v *VMIDMapping) GetByID(vmid int) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	name, ok := v.idToName[vmid]
	return name, ok
}

// Delete removes a mapping
func (v *VMIDMapping) Delete(vmid int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if name, ok := v.idToName[vmid]; ok {
		delete(v.nameToID, name)
	}
	delete(v.idToName, vmid)
}
