package internal

import (
	"fmt"
	"path/filepath"
)

const (
	alloyImage         = "grafana/alloy:latest" // You might want to pin this to a specific version
	alloyContainerName = "grafana-alloy"
	alloyConfigPath    = "/etc/alloy/config.alloy"
)

// CreateGrafanaAlloyService adds a Grafana Alloy service to the service manager.
func CreateGrafanaAlloyService(svcManager *Manifest, outDir *output, userConfigFile string) error {
	// Ensure the userConfigFile is an absolute path to be used for volume mounting.
	absUserConfigFile, err := filepath.Abs(userConfigFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for alloy config: %w", err)
	}

	alloySvc := svcManager.NewService(alloyContainerName)
	alloySvc.WithImage(alloyImage) // Set the image for the service
	alloySvc.WithArgs("run", alloyConfigPath)
	alloySvc.WithAbsoluteVolume(alloyConfigPath, absUserConfigFile) // Mount the absolute config path
	// alloySvc.AddPort(&Port{Name: "alloy-http", Port: 12345, HostPort: 12345, Protocol: ProtocolTCP}) // Example port, adjust as needed

	// Ensure Alloy joins the same network as other services.
	// This relies on the network being correctly set up by the runner.
	// If a specific network name is used by other services, ensure Alloy uses it too.
	// For now, we assume it will join the default bridge network created for the compose setup,
	// or the network specified by the --network flag which is handled by the NewLocalRunner.

	// Append the service directly, similar to how Prometheus service is added.
	svcManager.services = append(svcManager.services, alloySvc)

	return nil
}
