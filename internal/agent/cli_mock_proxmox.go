package agent

import (
	"fmt"
	"sync"
	"time"
)

// VM represents a virtual machine in the in-memory stub
type VM struct {
	ID        int
	Name      string
	State     VMState
	Cores     int
	Memory    int
	CloudInit string
}

// Task represents a task in the in-memory stub
type Task struct {
	ID         string
	Status     VMState
	ExitStatus string // "OK", "ERROR"
	StartTime  time.Time
	Type       string
	VMID       int
	NodeName   string
	User       string
}

// MockProxmoxClient implements ProxmoxClient interface for testing
type MockProxmoxClient struct {
	vms        map[int]*VM
	tasks      map[string]*Task
	nodeName   string
	lastTaskID int
	mu         sync.RWMutex
}

// NewMockProxmoxClient creates a new in-memory stub Proxmox client
func NewMockProxmoxClient(nodeName string) *MockProxmoxClient {
	return &MockProxmoxClient{
		vms:        make(map[int]*VM),
		tasks:      make(map[string]*Task),
		nodeName:   nodeName,
		lastTaskID: 0,
	}
}

// AddVM adds a VM to the stub client's state
func (c *MockProxmoxClient) AddVM(id int, name string, state VMState, cores int, memory int) *VM {
	c.mu.Lock()
	defer c.mu.Unlock()

	vm := &VM{
		ID:     id,
		Name:   name,
		State:  state,
		Cores:  cores,
		Memory: memory,
	}

	c.vms[id] = vm
	return vm
}

// GetVM retrieves a VM from the stub client's state
func (c *MockProxmoxClient) GetVM(id int) (*VM, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	vm, exists := c.vms[id]
	return vm, exists
}

// createTask creates a new task which is immediately completed
func (c *MockProxmoxClient) createTask(taskType string, vmID int, exitStatus string) *TaskResponse {
	// This method is called from methods that already have the mutex locked
	// so we don't need to lock again
	c.lastTaskID++

	// Format similar to a Proxmox UPID
	taskID := fmt.Sprintf("UPID:%s:%x:%x:%x:%s:%d:root@pam:",
		c.nodeName, c.lastTaskID, time.Now().Unix(), time.Now().UnixNano(), taskType, vmID)

	task := &Task{
		ID:         taskID,
		Status:     VMStateStopped, // Task is immediately completed
		ExitStatus: exitStatus,
		StartTime:  time.Now(),
		Type:       taskType,
		VMID:       vmID,
		NodeName:   c.nodeName,
		User:       "root@pam",
	}

	c.tasks[taskID] = task

	return &TaskResponse{
		TaskID:    taskID,
		NodeName:  c.nodeName,
		PID:       fmt.Sprintf("%x", c.lastTaskID),
		PStart:    fmt.Sprintf("%x", time.Now().Unix()),
		StartTime: fmt.Sprintf("%x", time.Now().UnixNano()),
		Type:      taskType,
		ID:        fmt.Sprintf("%d", vmID),
		User:      "root@pam",
	}
}

// CloneVM creates a new VM by cloning from a template
func (c *MockProxmoxClient) CloneVM(_ int, newVMID int, name string) (*TaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, newVMExists := c.vms[newVMID]; newVMExists {
		return nil, fmt.Errorf("VM with ID %d already exists", newVMID)
	}

	// Clone the VM synchronously
	c.vms[newVMID] = &VM{
		ID:     newVMID,
		Name:   name,
		State:  VMStateStopped,
		Cores:  2,
		Memory: 2048,
	}

	// Create a completed task
	return c.createTask("qmclone", newVMID, "OK"), nil
}

// ConfigureVM configures a VM (CPU, memory, cloud-init)
func (c *MockProxmoxClient) ConfigureVM(vmID int, cores int, memory int, cloudInitConfig string) (*TaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	vm, exists := c.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM with ID %d not found", vmID)
	}

	// Configure the VM synchronously
	vm.Cores = cores
	vm.Memory = memory
	if cloudInitConfig != "" {
		vm.CloudInit = cloudInitConfig
	}

	// Create a completed task
	return c.createTask("qmconfig", vmID, "OK"), nil
}

// StartVM starts a virtual machine
func (c *MockProxmoxClient) StartVM(vmID int) (*TaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	vm, exists := c.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM with ID %d not found", vmID)
	}

	if vm.State == VMStateRunning {
		return nil, fmt.Errorf("VM with ID %d is already running", vmID)
	}

	// Start the VM synchronously
	vm.State = VMStateRunning

	// Create a completed task
	return c.createTask("qmstart", vmID, "OK"), nil
}

// StopVM stops a virtual machine
func (c *MockProxmoxClient) StopVM(vmID int) (*TaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	vm, exists := c.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM with ID %d not found", vmID)
	}

	if vm.State == VMStateStopped {
		return nil, fmt.Errorf("VM with ID %d is already stopped", vmID)
	}

	// Stop the VM synchronously
	vm.State = VMStateStopped

	// Create a completed task
	return c.createTask("qmstop", vmID, "OK"), nil
}

// DeleteVM deletes a virtual machine
func (c *MockProxmoxClient) DeleteVM(vmID int) (*TaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, exists := c.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM with ID %d not found", vmID)
	}

	// Delete the VM synchronously
	delete(c.vms, vmID)

	// Create a completed task
	return c.createTask("qmdel", vmID, "OK"), nil
}

// GetTaskStatus retrieves the current status of a task
func (c *MockProxmoxClient) GetTaskStatus(taskID string) (*TaskStatus, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	task, exists := c.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task with ID %s not found", taskID)
	}

	return &TaskStatus{
		ExitStatus: task.ExitStatus,
		Status:     task.Status,
		Node:       task.NodeName,
		PID:        c.lastTaskID,
		Type:       task.Type,
		ID:         fmt.Sprintf("%d", task.VMID),
		User:       task.User,
		StartTime:  task.StartTime.Unix(),
		UpID:       taskID,
	}, nil
}

// WaitForTask waits for a task to complete and returns the task's status
// Since all tasks are completed immediately in this stub, this just returns the task status
func (c *MockProxmoxClient) WaitForTask(taskID string, timeout time.Duration) (*TaskStatus, error) {
	return c.GetTaskStatus(taskID)
}

// GetVMInfo retrieves the current status of a virtual machine
func (c *MockProxmoxClient) GetVMInfo(vmID int) (*VMInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	vm, exists := c.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM with ID %d not found", vmID)
	}

	// Mock VM status with reasonable defaults
	status := &VMInfo{
		Name:      vm.Name,
		State:     vm.State,
		VMID:      vm.ID,
		NodeName:  c.nodeName,
		CPU:       0.0, // 0% CPU usage when not running
		CPUCount:  vm.Cores,
		Memory:    0,                              // No memory usage when not running
		MaxMemory: int64(vm.Memory) * 1024 * 1024, // Convert MB to bytes
		Disk:      1024 * 1024 * 1024,             // 1GB disk usage (mock value)
		MaxDisk:   10 * 1024 * 1024 * 1024,        // 10GB max disk (mock value)
		Uptime:    0,                              // No uptime when not running
		QMPStatus: "unknown",                      // QMP status (mock value)
	}

	// If VM is running, simulate some resource usage
	if vm.State == VMStateRunning {
		status.CPU = 0.05                    // 5% CPU usage
		status.Memory = status.MaxMemory / 4 // Using 25% of allocated memory
		status.Uptime = 3600                 // 1 hour uptime (mock value)
	}

	return status, nil
}
