package agent

// ProxmoxClient defines the interface for interacting with Proxmox VE API
type ProxmoxClient interface {
	// CloneVM creates a new VM by cloning from a template
	CloneVM(templateID int, newVMID int, name string) (*VMCloneResponse, error)

	// ConfigureVM configures a VM (CPU, memory, cloud-init)
	ConfigureVM(vmID int, cores int, memory int, cloudInitConfig string) error

	// StartVM starts a virtual machine
	StartVM(vmID int) error

	// StopVM stops a virtual machine
	StopVM(vmID int) error

	// DeleteVM deletes a virtual machine
	DeleteVM(vmID int) error

	// GetVMInfo gets information about a VM
	GetVMInfo(vmID int) (*VMInfo, error)

	// GetVMNetworkInterfaces gets network interface information from a VM
	GetVMNetworkInterfaces(vmID int) ([]VMNetworkInterface, error)

	// GetVMStatus gets the current status of a VM
	GetVMStatus(vmID int) (*VMStatus, error)

	// WaitForVMStatus waits for a VM to reach a specific status
	WaitForVMStatus(vmID int, status string, timeoutSec int) error
}

// VMSize represents the available VM sizes
type VMSize string

const (
	VMSizeS1 VMSize = "s1" // 1 core, 1GB RAM
	VMSizeS2 VMSize = "s2" // 2 cores, 2GB RAM
	VMSizeS4 VMSize = "s4" // 4 cores, 4GB RAM
)

// VMSizeConfig maps VM sizes to their resource configurations
var VMSizeConfig = map[VMSize]struct {
	Cores  int
	Memory int
}{
	VMSizeS1: {Cores: 1, Memory: 1024},
	VMSizeS2: {Cores: 2, Memory: 2048},
	VMSizeS4: {Cores: 4, Memory: 4096},
}

// VMCloneResponse represents the response from cloning a VM
type VMCloneResponse struct {
	TaskID string `json:"taskid"`
	VMID   int    `json:"vmid"`
}

// VMInfo represents the information about a VM
type VMInfo struct {
	ID     int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Memory int     `json:"maxmem"`
	Disk   int     `json:"maxdisk"`
	Node   string  `json:"node"`
}

// VMStatus represents the current status of a VM
type VMStatus struct {
	Status string `json:"status"` // running, stopped, etc.
	VMID   int    `json:"vmid"`
	Node   string `json:"node"`
}

// VMNetworkInterface represents a network interface in a VM
type VMNetworkInterface struct {
	Name        string   `json:"name"`
	IPAddresses []string `json:"ip-addresses"`
	MACAddress  string   `json:"hardware-address"`
}

// CloudInitConfig represents a cloud-init configuration for a VM
type CloudInitConfig struct {
	Hostname   string   `json:"hostname"`
	Username   string   `json:"user"`
	Password   string   `json:"password"`
	SSHKeys    []string `json:"ssh_authorized_keys"`
	JoinConfig *struct {
		JoinURL           string `json:"join_url"`
		JoinToken         string `json:"join_token"`
		JoinTokenHash     string `json:"join_token_hash"`
		KubernetesVersion string `json:"kubernetes_version"`
	} `json:"join_config,omitempty"`
}

// TaskStatus represents the status of a Proxmox task
type TaskStatus struct {
	ExitStatus string `json:"exitstatus"`
	Status     string `json:"status"`
}
