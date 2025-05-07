package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// CreateGrafanaAlloyServices creates a service running Grafana Alloy for observability
// that collects metrics, logs, and traces from all services.
func CreateGrafanaAlloyServices(manifest *Manifest, out *output) error {
	// Try to load environment variables from .env.grafana file
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	envFilePath := filepath.Join(cwd, ".env.grafana")

	// Load .env.grafana file if it exists (silently continues if file doesn't exist)
	_ = godotenv.Load(envFilePath)

	// Check for required environment variables
	requiredEnvVars := []string{
		"GRAFANA_REMOTE_URL",
		"GRAFANA_INSTANCE_ID",
		"GRAFANA_REMOTE_USERNAME",
		"GRAFANA_REMOTE_PASSWORD",
		"GRAFANA_METRICS_URL",
		"GRAFANA_METRICS_USERNAME",
		"GRAFANA_METRICS_PASSWORD",
		"GRAFANA_LOGS_URL",
		"GRAFANA_LOGS_USERNAME",
		"GRAFANA_LOGS_PASSWORD",
		"GRAFANA_TRACES_URL",
		"GRAFANA_TRACES_USERNAME",
		"GRAFANA_TRACES_PASSWORD",
	}

	// Check if all required environment variables are set
	missingVars := []string{}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			missingVars = append(missingVars, envVar)
		}
	}

	if len(missingVars) > 0 {
		// Create a helpful error message with instructions
		errorMsg := fmt.Sprintf("Missing required environment variables for Grafana Alloy: %s\n\n",
			strings.Join(missingVars, ", "))
		errorMsg += "Please either:\n"
		errorMsg += "1. Set these environment variables in your shell, or\n"
		errorMsg += fmt.Sprintf("2. Create a .env.grafana file in %s with these variables in KEY=VALUE format\n", out.dst)
		return fmt.Errorf(errorMsg)
	}

	// Create the Grafana Alloy configuration
	alloyConfig := `
// Remote configuration
remotecfg {
	url            = sys.env("GRAFANA_REMOTE_URL")
	id             = sys.env("GRAFANA_INSTANCE_ID")
	poll_frequency = "10s"

	basic_auth {
		username = sys.env("GRAFANA_REMOTE_USERNAME")
		password = sys.env("GRAFANA_REMOTE_PASSWORD")
	}
}

// Metrics
prometheus.remote_write "metrics_service" {
	endpoint {
		url = sys.env("GRAFANA_METRICS_URL")

		basic_auth {
			username = sys.env("GRAFANA_METRICS_USERNAME")
			password = sys.env("GRAFANA_METRICS_PASSWORD")
		}
	}
}

// Logs
loki.write "grafana_cloud_loki" {
	endpoint {
		url = sys.env("GRAFANA_LOGS_URL")

		basic_auth {
			username = sys.env("GRAFANA_LOGS_USERNAME")
			password = sys.env("GRAFANA_LOGS_PASSWORD")
		}
	}
}

// Traces
otelcol.receiver.otlp "otlp_receiver" {
  grpc {
    endpoint = "0.0.0.0:4317"
  }
  http {
    endpoint = "0.0.0.0:4318"
  }

  output {
    traces = [otelcol.exporter.otlp.grafanacloud.input]
  }
}

// Scrape metrics from services
prometheus.scrape "default" {
  targets = [
`

	// Add a scrape target for each service that exposes metrics
	scrapeTargets := []string{}
	for _, service := range manifest.services {
		for _, port := range service.Ports {
			if port.Name == "metrics" {
				metricsPath := "/metrics"
				if overrideMetricsPath, ok := service.Labels["metrics_path"]; ok {
					metricsPath = overrideMetricsPath
				}

				// Create a scrape target for the service
				scrapeTarget := fmt.Sprintf("    // %s\n    {__address__ = \"%s:%d\", __metrics_path__ = \"%s\"},",
					service.Name, service.Name, port.Port, metricsPath)
				scrapeTargets = append(scrapeTargets, scrapeTarget)
			}
		}
	}

	// If no service exposes metrics, add a default target
	if len(scrapeTargets) == 0 {
		scrapeTargets = append(scrapeTargets, "    // Default\n    {__address__ = \"localhost:5555\"}")
	}

	// Add the targets to the alloy config - ensure proper comma formatting
	alloyConfig += strings.Join(scrapeTargets, "") + "\n"

	// Continue the alloy config
	alloyConfig += `  ]
  forward_to = [prometheus.remote_write.metrics_service.receiver]
  scrape_interval = "10s"  // cannot be lower than poll_frequency
  scrape_timeout = "1s"  // should be lower than scrape_interval
}

// Export traces
otelcol.exporter.otlp "grafanacloud" {
  client {
    endpoint = sys.env("GRAFANA_TRACES_URL")
    auth = otelcol.auth.basic.grafanacloud.handler
  }
}

otelcol.auth.basic "grafanacloud" {
  username = sys.env("GRAFANA_TRACES_USERNAME")
  password = sys.env("GRAFANA_TRACES_PASSWORD")
}

// Collect logs from Docker containers
discovery.docker "linux" {
  host = "unix:///var/run/docker.sock"
}

loki.source.docker "container_logs" {
  host = "unix:///var/run/docker.sock"
  
  targets = discovery.docker.linux.targets 
    
  forward_to = [loki.write.grafana_cloud_loki.receiver]
}
`

	// Write the alloy configuration file
	if err := out.WriteFile("alloy.river", alloyConfig); err != nil {
		return fmt.Errorf("failed to write alloy.river: %w", err)
	}

	// Add Grafana Alloy service to the manifest
	srv := manifest.NewService("grafana-alloy").
		WithImage("grafana/alloy").
		WithTag("latest").
		WithArgs("run", "/etc/alloy/alloy.river").
		// Metrics port
		WithPort("metrics", 4000, "tcp").
		// OTLP gRPC port
		WithPort("otlp-grpc", 4317, "tcp").
		// OTLP HTTP port
		WithPort("otlp-http", 4318, "tcp").
		// Mount the alloy config file
		WithArtifact("/etc/alloy/alloy.river", "alloy.river").
		// Mount Docker socket for container discovery
		WithAbsoluteVolume("/var/run/docker.sock", "/var/run/docker.sock")

	// Add environment variables with values from environment
	for _, envVar := range requiredEnvVars {
		srv.WithEnv(envVar, os.Getenv(envVar))
	}

	srv.ComponentName = "null" // For now, later on we can create a Grafana Alloy component
	manifest.services = append(manifest.services, srv)

	return nil
}
