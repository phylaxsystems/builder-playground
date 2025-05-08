package internal

import (
	"fmt"
	"log"
	"slices"
	"strings"
)

func CreateCaddyServices(exposedServices []string, manifest *Manifest, out *output) error {
	// Create a Caddyfile configuration for reverse proxying all services with HTTP or WS ports
	var routes []string
	manifest.ctx.CaddyEnabled = true

	bodyContent := "Available services:\\n"

	exposedWithPorts := map[string]bool{}

	// Add a routes for each service with http or ws ports
	for _, service := range manifest.services {
		if slices.Contains(exposedServices, service.Name) {
			for _, port := range service.Ports {
				// Check if this port is likely to be HTTP or WebSocket
				isHttpPort := port.Name == "http" || port.Name == "ws" ||
					strings.Contains(port.Name, "http") ||
					strings.Contains(port.Name, "rpc") ||
					port.Port == 8545 || // Common Ethereum RPC port
					port.Port == 8546 || // Common Ethereum WebSocket port
					port.Port == 8080 || // Common HTTP port
					port.Port == 3000 || // Common web app port
					port.Port == 3500 || // Beacon node HTTP port
					port.Port == 5555 || // MevBoost port
					port.Port == 8549 // op-node HTTP port

				if isHttpPort {
					// Create a route for the service with port type in the path
					// Format: /<service-name>/<port-type>/* -> http://<service-name>:<port>/{path}
					route := fmt.Sprintf("  handle_path /%s/%s {\n", service.Name, port.Name)
					route += fmt.Sprintf("    uri strip_prefix /%s/%s\n", service.Name, port.Name)
					route += fmt.Sprintf("    reverse_proxy %s:%d\n", service.Name, port.Port)
					route += "  }\n\n"
					// Add the service to the body content of the index page
					bodyContent += fmt.Sprintf("%s (%s): /%s/%s\\n", service.Name, port.Name, service.Name, port.Name)
					routes = append(routes, route)
					exposedWithPorts[service.Name] = true
				}
			}

			// If this service was requested but we didn't find any HTTP/WS ports, log it
			if !exposedWithPorts[service.Name] {
				log.Printf("Warning: Service %s was requested to be exposed in Caddy, but no HTTP or WS ports were found", service.Name)
			}
		}
	}

	if len(routes) == 0 {
		// No HTTP or WS services to proxy, skip creating Caddy
		missingPorts := []string{}
		for _, svc := range exposedServices {
			if !exposedWithPorts[svc] {
				missingPorts = append(missingPorts, svc)
			}
		}

		if len(missingPorts) > 0 {
			return fmt.Errorf("no HTTP or WS services to proxy. The following services do not have HTTP or WS ports: %s", strings.Join(missingPorts, ", "))
		}

		return fmt.Errorf("no HTTP or WS services to proxy")
	}

	// Create the Caddyfile
	caddyfile := ":8888 {\n"
	caddyfile += "  # Automatically generated routes for HTTP and WebSocket services\n\n"

	// Add a root route that lists available services
	caddyfile += "  # Root route showing available services\n"
	caddyfile += "  respond / 200 {\n"
	caddyfile += fmt.Sprintf("    body %q\n", bodyContent)
	caddyfile += "  }\n\n"

	// Add all the routes
	for _, route := range routes {
		caddyfile += route
	}

	caddyfile += "}\n"

	// Write the Caddyfile
	if err := out.WriteFile("Caddyfile", caddyfile); err != nil {
		return fmt.Errorf("failed to write Caddyfile: %w", err)
	}

	// Add Caddy service to the manifest
	srv := manifest.NewService("caddy").
		WithImage("caddy").
		WithTag("2").
		WithPort("http", 8888, "tcp").
		WithArtifact("/etc/caddy/Caddyfile", "Caddyfile")

	// Add the service to the manifest
	srv.ComponentName = "null" // Using null since there's no dedicated Caddy component
	manifest.services = append(manifest.services, srv)

	log.Printf("Successfully created Caddy proxy for services: %v", exposedWithPorts)

	return nil
}
