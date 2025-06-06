package agent

import (
	"time"
)

// ProxmoxClient defines the interface for interacting with Proxmox VE API
type ProxmoxClient interface {
	// CloneVM creates a new VM by cloning from a template
	CloneVM(templateID int, newVMID int, name string) (*TaskResponse, error)

	// ConfigureVM configures a VM (CPU, memory, cloud-init)
	ConfigureVM(vmID int, cores int, memory int, cloudInitConfig string) (*TaskResponse, error)

	// StartVM starts a virtual machine
	StartVM(vmID int) (*TaskResponse, error)

	// StopVM stops a virtual machine
	StopVM(vmID int) (*TaskResponse, error)

	// DeleteVM deletes a virtual machine
	DeleteVM(vmID int) (*TaskResponse, error)

	// WaitForTask waits for a task to complete and returns the task's status
	WaitForTask(taskID string, timeout time.Duration) (*TaskStatus, error)

	// GetTaskStatus retrieves the current status of a task
	GetTaskStatus(taskID string) (*TaskStatus, error)

	// GetVMInfo retrieves the current status of a virtual machine
	GetVMInfo(vmID int) (*VMInfo, error)
}

// TaskResponse represents a Proxmox API response containing a task ID
type TaskResponse struct {
	TaskID    string `json:"taskid"`    // The original UPID string
	NodeName  string `json:"node"`      // Node name where the task is running
	PID       string `json:"pid"`       // Process ID in hex
	PStart    string `json:"pstart"`    // Process start time in hex
	StartTime string `json:"starttime"` // Start time of the task in hex
	Type      string `json:"type"`      // Task type
	ID        string `json:"id"`        // Optional ID field
	User      string `json:"user"`      // User@Realm who initiated the task
}

// TaskStatus represents the status of a Proxmox task
type TaskStatus struct {
	ExitStatus string   `json:"exitstatus"` // Such as 'OK' or 'ERROR'
	Status     VMStatus `json:"status"`     // Such as 'stopped' or 'running'
	Node       string   `json:"node"`       // Node name where the task is running
	PID        int      `json:"pid"`        // Process ID
	Type       string   `json:"type"`       // Task type
	ID         string   `json:"id"`         // Optional ID
	User       string   `json:"user"`       // User@realm who initiated the task
	StartTime  int64    `json:"starttime"`  // Start time of the task
	UpID       string   `json:"upid"`       // Full UPID of the task
}

type VMStatus string

const (
	VMStatusRunning VMStatus = "running"
	VMStatusStopped VMStatus = "stopped"
	VMStatusPaused  VMStatus = "paused"
	VMStatusError   VMStatus = "error"
	VMStatusUnknown VMStatus = "unknown"
)

// VMInfo represents the status of a Proxmox virtual machine
type VMInfo struct {
	Name      string   `json:"name"`      // VM name
	Status    VMStatus `json:"status"`    // Such as 'running' or 'stopped'
	VMID      int      `json:"vmid"`      // VM ID
	NodeName  string   `json:"node"`      // Node name where the VM is running
	CPU       float64  `json:"cpu"`       // Current CPU usage (0-1 range)
	CPUCount  int      `json:"cpus"`      // Number of CPUs
	Memory    int64    `json:"mem"`       // Current memory usage in bytes
	MaxMemory int64    `json:"maxmem"`    // Maximum memory in bytes
	Disk      int64    `json:"disk"`      // Disk usage in bytes
	MaxDisk   int64    `json:"maxdisk"`   // Maximum disk size in bytes
	Uptime    int64    `json:"uptime"`    // Uptime in seconds
	QMPStatus string   `json:"qmpstatus"` // QEMU Machine Protocol status
}
