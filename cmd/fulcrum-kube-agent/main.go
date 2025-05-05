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
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Builder().LoadFile(configPath).WithEnv(".").Build()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Println("Starting agent ...")

	// Clients
	fulcrumCli := fulcrum.NewFulcrumClient(cfg.FulcrumAPIURL, cfg.AgentToken, httpcli.WithSkipTLSVerify(cfg.SkipTLSVerify))

	// Create and start the agent
	testAgent, err := agent.New(fulcrumCli)
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
