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
	Status    string // "running", "stopped"
	Cores     int
	Memory    int
	CloudInit string
}

// Task represents a task in the in-memory stub
type Task struct {
	ID         string
	Status     string // "running", "stopped"
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
func (c *MockProxmoxClient) AddVM(id int, name string, status string, cores int, memory int) *VM {
	c.mu.Lock()
	defer c.mu.Unlock()

	vm := &VM{
		ID:     id,
		Name:   name,
		Status: status,
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
		Status:     "stopped", // Task is immediately completed
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
func (c *MockProxmoxClient) CloneVM(templateID int, newVMID int, name string) (*TaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	template, templateExists := c.vms[templateID]

	if !templateExists {
		return nil, fmt.Errorf("template VM with ID %d not found", templateID)
	}

	_, newVMExists := c.vms[newVMID]

	if newVMExists {
		return nil, fmt.Errorf("VM with ID %d already exists", newVMID)
	}

	// Clone the VM synchronously
	c.vms[newVMID] = &VM{
		ID:     newVMID,
		Name:   name,
		Status: "stopped",
		Cores:  template.Cores,
		Memory: template.Memory,
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

	if vm.Status == "running" {
		return nil, fmt.Errorf("VM with ID %d is already running", vmID)
	}

	// Start the VM synchronously
	vm.Status = "running"

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

	if vm.Status == "stopped" {
		return nil, fmt.Errorf("VM with ID %d is already stopped", vmID)
	}

	// Stop the VM synchronously
	vm.Status = "stopped"

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
