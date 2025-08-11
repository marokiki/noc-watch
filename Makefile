.PHONY: build install uninstall enable start stop status logs

# Binary name
BINARY=noc-watch
SERVICE=noc-watch.service

# Installation paths
BIN_DIR=/usr/local/bin
SERVICE_DIR=/etc/systemd/system
LOG_DIR=/var/log/noc-watch

# Build the binary
build:
	go build -o $(BINARY) .

# Install binary and service
install: build
	@echo "Installing $(BINARY)..."
	sudo install -m 755 $(BINARY) $(BIN_DIR)/
	@echo "Installing systemd service..."
	sudo install -m 644 $(SERVICE) $(SERVICE_DIR)/
	@echo "Creating log directory..."
	sudo mkdir -p $(LOG_DIR)
	sudo chown root:root $(LOG_DIR)
	sudo chmod 755 $(LOG_DIR)
	@echo "Reloading systemd..."
	sudo systemctl daemon-reload
	@echo "Installation complete!"

# Uninstall binary and service
uninstall:
	@echo "Stopping service..."
	sudo systemctl stop $(SERVICE) || true
	@echo "Disabling service..."
	sudo systemctl disable $(SERVICE) || true
	@echo "Removing binary..."
	sudo rm -f $(BIN_DIR)/$(BINARY)
	@echo "Removing service file..."
	sudo rm -f $(SERVICE_DIR)/$(SERVICE)
	@echo "Reloading systemd..."
	sudo systemctl daemon-reload
	@echo "Uninstallation complete!"

# Enable and start the service
enable:
	@echo "Enabling service..."
	sudo systemctl enable $(SERVICE)
	@echo "Starting service..."
	sudo systemctl start $(SERVICE)

# Start the service
start:
	sudo systemctl start $(SERVICE)

# Stop the service
stop:
	sudo systemctl stop $(SERVICE)

# Check service status
status:
	sudo systemctl status $(SERVICE)

# View service logs
logs:
	sudo journalctl -u $(SERVICE) -f

# View log file
logfile:
	@echo "Log file contents:"
	@sudo cat $(LOG_DIR)/wifi_quality_log.txt || echo "Log file not found"

# Clean build artifacts
clean:
	rm -f $(BINARY)
