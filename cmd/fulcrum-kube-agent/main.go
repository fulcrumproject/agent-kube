package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fulcrumproject.org/kube-agent/internal/agent"
	"fulcrumproject.org/kube-agent/internal/config"
	"fulcrumproject.org/kube-agent/internal/fulcrum"
	"fulcrumproject.org/kube-agent/internal/httpcli"
	"fulcrumproject.org/kube-agent/internal/kamaji"
	"fulcrumproject.org/kube-agent/internal/proxmox"
	"fulcrumproject.org/kube-agent/internal/ssh"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	useMock := flag.Bool("mock", false, "Use mock implementations of clients")
	flag.Parse()

	cfg, err := config.Builder().LoadFile(configPath).WithEnv().Build()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Println("Starting agent ...")

	// Initialize clients based on the mock flag
	var clients *agent.Clients
	if *useMock {
		log.Println("Using mock clients...")
		clients = initMockClients(cfg)
	} else {
		clients = initRealClients(cfg)
	}
	defer clients.Close()

	// Create and start the agent with all required clients
	testAgent, err := agent.New(clients, cfg.JobPollInterval, cfg.MetricReportInterval)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start the agent
	if err := testAgent.Start(ctx); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	log.Printf("Agent started successfully (Agent ID: %s)", testAgent.GetAgentID())
	log.Printf("Press Ctrl+C to stop the agent")

	// Wait for termination signal
	<-sigCh
	log.Println("Received shutdown signal")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shut down the agent
	if err := testAgent.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Error during shutdown: %v", err)
	}

	log.Println("Agent shutdown succesfully.")
}

func initRealClients(cfg *config.Config) *agent.Clients {
	// Fulcrum client for communicating with the Fulcrum Core API
	fulcrumCli := fulcrum.NewFulcrumClient(cfg.FulcrumAPIURL, cfg.FulcrumAPIToken, httpcli.WithSkipTLSVerify(cfg.SkipTLSVerify))

	// Proxmox client for VM management
	proxmoxHttpClient := httpcli.NewHTTPClient(cfg.ProxmoxAPIURL, cfg.ProxmoxAPIToken, httpcli.WithSkipTLSVerify(cfg.SkipTLSVerify))
	proxmoxCli := proxmox.NewProxmoxClient(cfg.ProxmoxHost, cfg.ProxmoxStorage, proxmoxHttpClient)

	// Kamaji client for Kubernetes tenant control planes
	kamajiCli, err := kamaji.NewClient(cfg.KubeAPIURL, cfg.KubeAPIToken)
	if err != nil {
		log.Fatalf("Failed to create Kamaji client: %v", err)
	}

	// SSH client for SCP operations (Cloud-Init templates)
	sshOpts := ssh.Options{
		Host:           cfg.ProxmoxCIHost,
		Username:       cfg.ProxmoxCIUser,
		PrivateKeyPath: cfg.ProxmoxCIPKPath,
		Timeout:        30 * time.Second,
	}

	sshCli, err := ssh.NewClient(sshOpts)
	if err != nil {
		log.Fatalf("Failed to create SSH client: %v", err)
	}
	return &agent.Clients{
		Fulcrum: fulcrumCli,
		Proxmox: proxmoxCli,
		Kamaji:  kamajiCli,
		SSH:     sshCli,
	}
}

// initMockClients creates mock implementations for testing
func initMockClients(cfg *config.Config) *agent.Clients {
	// Fulcrum client for communicating with the Fulcrum Core API
	fulcrumCli := fulcrum.NewFulcrumClient(cfg.FulcrumAPIURL, cfg.FulcrumAPIToken, httpcli.WithSkipTLSVerify(cfg.SkipTLSVerify))

	return &agent.Clients{
		Fulcrum: fulcrumCli,
		Proxmox: agent.NewMockProxmoxClient("mock-node"),
		Kamaji:  agent.NewMockKamajiClient(),
		SSH:     agent.NewMockSSHClient(),
	}
}
