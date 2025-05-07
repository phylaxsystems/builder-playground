package internal

import (
	"fmt"
	"strings"
)

// CreateGrafanaAlloyServices creates a service running Grafana Alloy for observability
// that collects metrics, logs, and traces from all services.
func CreateGrafanaAlloyServices(manifest *Manifest, out *output) error {
	// Create the Grafana Alloy configuration
	alloyConfig := `
# Remote configuration
remotecfg {
	url            = "${GRAFANA_REMOTE_URL}"
	id             = "${GRAFANA_INSTANCE_ID}"
	poll_frequency = "10s"

	basic_auth {
		username = "${GRAFANA_REMOTE_USERNAME}"
		password = "${GRAFANA_REMOTE_PASSWORD}"
	}
}

# Metrics
prometheus.remote_write "metrics_service" {
	endpoint {
		url = "${GRAFANA_METRICS_URL}"

		basic_auth {
			username = "${GRAFANA_METRICS_USERNAME}"
			password = "${GRAFANA_METRICS_PASSWORD}"
		}
	}
}

# Logs
loki.write "grafana_cloud_loki" {
	endpoint {
		url = "${GRAFANA_LOGS_URL}"

		basic_auth {
			username = "${GRAFANA_LOGS_USERNAME}"
			password = "${GRAFANA_LOGS_PASSWORD}"
		}
	}
}

# Traces
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

# Scrape metrics from services
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
				scrapeTarget := fmt.Sprintf("    # %s\n    {\"__address__\" = \"%s:%d\", \"__metrics_path__\" = \"%s\"}",
					service.Name, service.Name, port.Port, metricsPath)
				scrapeTargets = append(scrapeTargets, scrapeTarget)
			}
		}
	}

	// If no service exposes metrics, add a default target
	if len(scrapeTargets) == 0 {
		scrapeTargets = append(scrapeTargets, "    # Default\n    {\"__address__\" = \"localhost:5555\"}")
	}

	// Add the targets to the alloy config
	alloyConfig += strings.Join(scrapeTargets, ",\n") + "\n"

	// Continue the alloy config
	alloyConfig += `  ]
  forward_to = [prometheus.remote_write.metrics_service.receiver]
  scrape_interval = "1s"
  scrape_timeout = "1s"
}

# Export traces
otelcol.exporter.otlp "grafanacloud" {
  client {
    endpoint = "${GRAFANA_TRACES_URL}"
    auth = otelcol.auth.basic.grafanacloud.handler
  }
}

otelcol.auth.basic "grafanacloud" {
  username = "${GRAFANA_TRACES_USERNAME}"
  password = "${GRAFANA_TRACES_PASSWORD}"
}

# Collect logs from Docker containers
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

	// Add to the manifest the Grafana Alloy service
	srv := manifest.NewService("grafana-alloy").
		WithImage("grafana/alloy").
		WithTag("latest").
		WithArgs("-config.file", "/etc/alloy/alloy.river").
		// Metrics port
		WithPort("metrics", 4000, "tcp").
		// OTLP gRPC port
		WithPort("otlp-grpc", 4317, "tcp").
		// OTLP HTTP port
		WithPort("otlp-http", 4318, "tcp").
		// Mount the alloy config file
		WithArtifact("/etc/alloy/alloy.river", "alloy.river").
		// Mount Docker socket for container discovery
		WithAbsoluteVolume("/var/run/docker.sock", "/var/run/docker.sock").
		// Environment variables for sensitive information
		WithEnv("GRAFANA_REMOTE_URL", "${GRAFANA_REMOTE_URL}").
		WithEnv("GRAFANA_INSTANCE_ID", "${GRAFANA_INSTANCE_ID}").
		WithEnv("GRAFANA_REMOTE_USERNAME", "${GRAFANA_REMOTE_USERNAME}").
		WithEnv("GRAFANA_REMOTE_PASSWORD", "${GRAFANA_REMOTE_PASSWORD}").
		WithEnv("GRAFANA_METRICS_URL", "${GRAFANA_METRICS_URL}").
		WithEnv("GRAFANA_METRICS_USERNAME", "${GRAFANA_METRICS_USERNAME}").
		WithEnv("GRAFANA_METRICS_PASSWORD", "${GRAFANA_METRICS_PASSWORD}").
		WithEnv("GRAFANA_LOGS_URL", "${GRAFANA_LOGS_URL}").
		WithEnv("GRAFANA_LOGS_USERNAME", "${GRAFANA_LOGS_USERNAME}").
		WithEnv("GRAFANA_LOGS_PASSWORD", "${GRAFANA_LOGS_PASSWORD}").
		WithEnv("GRAFANA_TRACES_URL", "${GRAFANA_TRACES_URL}").
		WithEnv("GRAFANA_TRACES_USERNAME", "${GRAFANA_TRACES_USERNAME}").
		WithEnv("GRAFANA_TRACES_PASSWORD", "${GRAFANA_TRACES_PASSWORD}")

	srv.ComponentName = "null" // For now, later on we can create a Grafana Alloy component
	manifest.services = append(manifest.services, srv)

	return nil
}
