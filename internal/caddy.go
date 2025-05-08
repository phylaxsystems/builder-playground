func CreateCaddyServices(manifest *Manifest, out *output) error {
	// Create a Caddyfile configuration for reverse proxying all services with HTTP or WS ports
	var routes []string
	manifest.ctx.CaddyEnabled = true

	// Add a routes for each service with http or ws ports
	for _, service := range manifest.services {
		for _, port := range service.Ports {
			// Only look for HTTP and WebSocket ports
			if port.Name == "http" || port.Name == "ws" {

				// Create a route for the service with port type in the path
				// Format: /<service-name>/<port-type>/* -> http://<service-name>:<port>/{path}
				route := fmt.Sprintf("  handle_path /%s/%s {\n", service.Name, port.Name)
				route += fmt.Sprintf("    uri strip_prefix /%s/%s\n", service.Name, port.Name)
				route += fmt.Sprintf("    reverse_proxy %s:%d\n", service.Name, port.Port)
				route += "  }\n\n"

				routes = append(routes, route)
			}
		}
	}

	if len(routes) == 0 {
		// No HTTP or WS services to proxy, skip creating Caddy
		return nil
	}

	// Create the Caddyfile
	caddyfile := ":8888 {\n"
	caddyfile += "  # Automatically generated routes for HTTP and WebSocket services\n\n"

	// Add a root route that lists available services
	caddyfile += "  # Root route showing available services\n"
	caddyfile += "  respond / 200 {\n"

	// Build a single body content string
	bodyContent := "Available services:\\n"

	for _, service := range manifest.services {
		for _, port := range service.Ports {
			if port.Name == "http" || port.Name == "ws" {
				bodyContent += fmt.Sprintf("%s (%s): /%s/%s\\n", service.Name, port.Name, service.Name, port.Name)
			}
		}
	}

	// Add the single body directive
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

	return nil
}