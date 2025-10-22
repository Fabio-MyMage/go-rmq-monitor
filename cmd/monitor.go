package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"go-rmq-monitor/internal/config"
	"go-rmq-monitor/internal/logger"
	"go-rmq-monitor/internal/monitor"
	"go-rmq-monitor/internal/pidfile"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Start monitoring RabbitMQ queues",
	Long:  `Continuously monitor RabbitMQ queues for stuck messages and log alerts.`,
	RunE:  runMonitor,
}

var (
	daemonMode bool
	verbose    int
)

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "Run in background (daemon mode)")
	monitorCmd.Flags().CountVarP(&verbose, "verbose", "v", "Increase verbosity (-v, -vv, -vvv)")
}

func runMonitor(cmd *cobra.Command, args []string) error {
	// Handle daemon mode
	if daemonMode {
		return runAsDaemon()
	}

	// Load configuration
	configPath := cfgFile
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create and lock PID file to prevent multiple instances
	pidFilePath := pidfile.GetDefaultPath(configPath)
	pid := pidfile.New(pidFilePath)
	if err := pid.Create(); err != nil {
		return fmt.Errorf("failed to create PID file: %w", err)
	}
	defer pid.Remove()

	// Adjust log level based on verbosity
	if verbose >= 3 {
		cfg.Logging.Level = "debug"
	} else if verbose == 2 {
		cfg.Logging.Level = "info"
	}

	// Initialize logger
	log, err := logger.New(cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Close()

	log.Info("Starting RabbitMQ monitor", map[string]interface{}{
		"vhost":    cfg.RabbitMQ.VHost,
		"interval": cfg.Monitor.Interval.String(),
		"host":     cfg.RabbitMQ.Host,
	})

	// Create monitor service
	monitorService, err := monitor.New(cfg, log, verbose)
	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- monitorService.Start()
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Info("Received shutdown signal", map[string]interface{}{"signal": sig.String()})
		monitorService.Stop()
	case err := <-errChan:
		if err != nil {
			log.Error("Monitor service error", err, nil)
			return err
		}
	}

	log.Info("Monitor stopped", nil)
	return nil
}

func runAsDaemon() error {
	// Re-execute the command without --daemon flag
	args := make([]string, 0)
	
	for _, arg := range os.Args[1:] {
		// Skip standalone --daemon and -d flags
		if arg == "--daemon" || arg == "-d" {
			continue
		}
		
		// Handle combined short flags like -dvvv
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			// This is a short flag (potentially combined)
			if arg[1] == 'd' {
				// Remove 'd' from the flag string
				remaining := ""
				for _, ch := range arg[1:] {
					if ch != 'd' {
						remaining += string(ch)
					}
				}
				// If there are other flags besides 'd', keep them
				if len(remaining) > 0 {
					args = append(args, "-"+remaining)
				}
				continue
			}
		}
		
		args = append(args, arg)
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	
	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("Monitor started in background (PID: %d)\n", cmd.Process.Pid)
	fmt.Printf("To stop: kill %d\n", cmd.Process.Pid)
	
	// Release the process
	cmd.Process.Release()
	
	return nil
}
