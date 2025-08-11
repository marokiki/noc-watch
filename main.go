package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/tview"
)

// WiFiTest represents a single WiFi quality test result
type WiFiTest struct {
	DHCPRenewTime    time.Duration // Time taken for DHCP renewal
	IPv4Connectivity bool          // IPv4 connectivity status
	IPv6Connectivity bool          // IPv6 connectivity status
	Latency          time.Duration // Measured latency
	Success          bool          // Overall test success status
	Timestamp        time.Time     // Test execution timestamp
}

// WiFiMonitor manages WiFi quality testing and UI updates
type WiFiMonitor struct {
	dhcpTests    []WiFiTest         // DHCP test history
	pingTests    []WiFiTest         // Ping test history
	successCount int                // Total successful tests
	totalCount   int                // Total tests executed
	app          *tview.Application // TUI application reference
	statsView    *tview.TextView    // Statistics display widget
	chartView    *tview.TextView    // Chart display widget
	logView      *tview.TextView    // Log display widget

	wifiInterface string // Network interface used for tests (e.g., wlan0)
}

// NewWiFiMonitor creates a new WiFi monitor instance
func NewWiFiMonitor() *WiFiMonitor {
	return &WiFiMonitor{
		dhcpTests:     make([]WiFiTest, 0),
		pingTests:     make([]WiFiTest, 0),
		wifiInterface: "wlan0",
	}
}

// runDHCPRenew performs DHCP release and renewal, measuring the time taken
func (w *WiFiMonitor) runDHCPRenew() (time.Duration, bool) {
	// Release current DHCP lease for the specific interface
	cmd := exec.Command("sudo", "dhclient", "-r", w.wifiInterface)
	cmd.Run()

	// Wait for network to settle
	time.Sleep(2 * time.Second)

	start := time.Now()
	// Request new DHCP lease for the specific interface
	cmd = exec.Command("sudo", "dhclient", w.wifiInterface)
	err := cmd.Run()

	if err != nil {
		return 0, false
	}

	// Verify DNS server configuration
	cmd = exec.Command("cat", "/etc/resolv.conf")
	output, err := cmd.Output()
	if err != nil {
		return 0, false
	}

	// Check if nameserver is configured
	if !strings.Contains(string(output), "nameserver") {
		return 0, false
	}

	return time.Since(start), true
}

// checkIPv4Connectivity tests IPv4 connectivity using Google DNS
func (w *WiFiMonitor) checkIPv4Connectivity() bool {
	cmd := exec.Command("ping", "-I", w.wifiInterface, "-c", "1", "-W", "5", "8.8.8.8")
	err := cmd.Run()
	return err == nil
}

// checkIPv6Connectivity tests IPv6 connectivity using Google DNS
func (w *WiFiMonitor) checkIPv6Connectivity() bool {
	cmd := exec.Command("ping6", "-I", w.wifiInterface, "-c", "1", "-W", "5", "2001:4860:4860::8888")
	err := cmd.Run()
	return err == nil
}

// measureLatency measures network latency using ping command
func (w *WiFiMonitor) measureLatency() time.Duration {
	start := time.Now()
	cmd := exec.Command("ping", "-I", w.wifiInterface, "-c", "3", "-W", "5", "8.8.8.8")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Extract average latency from ping output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "avg") {
			parts := strings.Split(line, "=")
			if len(parts) > 1 {
				latencyStr := strings.TrimSpace(strings.Split(parts[1], " ")[0])
				if latency, err := strconv.ParseFloat(latencyStr, 64); err == nil {
					return time.Duration(latency * float64(time.Millisecond))
				}
			}
		}
	}

	return time.Since(start)
}

// runTest executes a complete WiFi quality test
func (w *WiFiMonitor) runTest() WiFiTest {
	test := WiFiTest{
		Timestamp: time.Now(),
	}

	// DHCP renewal test
	dhcpTime, dhcpSuccess := w.runDHCPRenew()
	test.DHCPRenewTime = dhcpTime

	// Connectivity tests
	test.IPv4Connectivity = w.checkIPv4Connectivity()
	test.IPv6Connectivity = w.checkIPv6Connectivity()

	// Latency test
	test.Latency = w.measureLatency()

	// Determine overall success
	test.Success = dhcpSuccess && test.IPv4Connectivity && (test.Latency > 0)

	return test
}

// updateUI updates all UI components with current test data
func (w *WiFiMonitor) updateUI() {
	// Calculate success rate
	var successRate float64
	if w.totalCount > 0 {
		successRate = float64(w.successCount) / float64(w.totalCount) * 100
	}

	// Get current time
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// Update statistics display
	statsText := fmt.Sprintf(
		"[white]WiFi Quality Monitor - NOC Watch -\n"+
			"Current Time: [cyan]%s[white]\n"+
			"Total Tests: %d | [green]Success: %d[white] | [red]Failure: %d[white]\n"+
			"Success Rate: [yellow]%.2f%%[white]\n",
		currentTime, w.totalCount, w.successCount, w.totalCount-w.successCount, successRate,
	)

	// Update chart display (ASCII art)
	chartText := "Test Results:\n\n"

	// DHCP Test Results
	chartText += "[yellow]DHCP Test Results (Every 5 minutes):[white]\n"
	if len(w.dhcpTests) == 0 {
		chartText += "  [yellow]Waiting for first DHCP test...[white]\n"
	} else {
		for i, test := range w.dhcpTests {
			if i >= 10 { // Show only latest 10 DHCP tests
				break
			}
			status := "[red]✗"
			if test.Success {
				status = "[green]✓"
			}
			chartText += fmt.Sprintf("  [%d] %s DHCP: %v\n",
				i+1, status, test.DHCPRenewTime)
		}
	}

	chartText += "\n[yellow]Ping Test Results (Every 1 minute):[white]\n"
	if len(w.pingTests) == 0 {
		chartText += "  [yellow]Waiting for first ping test...[white]\n"
	} else {
		for i, test := range w.pingTests {
			if i >= 10 { // Show only latest 10 ping tests
				break
			}
			chartText += fmt.Sprintf("  [%d] IPv4: %v IPv6: %v Latency: %v\n",
				i+1, test.IPv4Connectivity, test.IPv6Connectivity, test.Latency)
		}
	}

	// Update log display
	logText := "Latest Test Results:\n\n"

	// Latest DHCP Test
	logText += "[yellow]Latest DHCP Test:[white]\n"
	if len(w.dhcpTests) > 0 {
		latest := w.dhcpTests[len(w.dhcpTests)-1]
		logText += fmt.Sprintf("Time: %s\n", latest.Timestamp.Format("15:04:05"))
		logText += fmt.Sprintf("DHCP Renew: %v\n", latest.DHCPRenewTime)
		logText += fmt.Sprintf("IPv4: %v\n", latest.IPv4Connectivity)
		logText += fmt.Sprintf("IPv6: %v\n", latest.IPv6Connectivity)
		logText += fmt.Sprintf("Latency: %v\n", latest.Latency)
		logText += fmt.Sprintf("Success: %v\n", latest.Success)
	} else {
		logText += "[yellow]No DHCP tests completed yet.[white]\n"
	}

	logText += "\n[yellow]Latest Ping Test:[white]\n"
	if len(w.pingTests) > 0 {
		latest := w.pingTests[len(w.pingTests)-1]
		logText += fmt.Sprintf("Time: %s\n", latest.Timestamp.Format("15:04:05"))
		logText += fmt.Sprintf("IPv4: %v\n", latest.IPv4Connectivity)
		logText += fmt.Sprintf("IPv6: %v\n", latest.IPv6Connectivity)
		logText += fmt.Sprintf("Latency: %v\n", latest.Latency)
		logText += fmt.Sprintf("Success: %v\n", latest.Success)
	} else {
		logText += "[yellow]No ping tests completed yet.[white]\n"
	}

	// Update UI components (thread-safe)
	w.app.QueueUpdateDraw(func() {
		w.statsView.SetText(statsText)
		w.chartView.SetText(chartText)
		w.logView.SetText(logText)
	})
}

// startMonitoring begins periodic WiFi quality testing
func (w *WiFiMonitor) startMonitoring() {
	// DHCP test every 5 minutes
	dhcpTicker := time.NewTicker(5 * time.Minute)
	defer dhcpTicker.Stop()

	// Ping and latency test every 1 minute
	pingTicker := time.NewTicker(1 * time.Minute)
	defer pingTicker.Stop()

	// UI update every 1 second for current time display
	uiTicker := time.NewTicker(1 * time.Second)
	defer uiTicker.Stop()

	// Initial UI update to show the framework
	w.updateUI()

	for {
		select {
		case <-dhcpTicker.C:
			// Run full test including DHCP renewal
			test := w.runTest()
			w.dhcpTests = append(w.dhcpTests, test)
			w.totalCount++

			if test.Success {
				w.successCount++
			}

			w.updateUI()

		case <-pingTicker.C:
			// Run only connectivity and latency tests (skip DHCP)
			test := w.runConnectivityTest()
			w.pingTests = append(w.pingTests, test)
			w.totalCount++

			if test.Success {
				w.successCount++
			}

			w.updateUI()

		case <-uiTicker.C:
			// Update UI every second for current time display
			w.updateUI()
		}
	}
}

// runConnectivityTest executes connectivity and latency tests without DHCP renewal
func (w *WiFiMonitor) runConnectivityTest() WiFiTest {
	test := WiFiTest{
		Timestamp: time.Now(),
	}

	// Skip DHCP renewal test
	test.DHCPRenewTime = 0

	// Connectivity tests
	test.IPv4Connectivity = w.checkIPv4Connectivity()
	test.IPv6Connectivity = w.checkIPv6Connectivity()

	// Latency test
	test.Latency = w.measureLatency()

	// Determine overall success (DHCP is not required for this test)
	test.Success = test.IPv4Connectivity && (test.Latency > 0)

	return test
}

func main() {
	monitor := NewWiFiMonitor()

	// Create TUI application
	app := tview.NewApplication()
	monitor.app = app

	// Create widgets
	monitor.statsView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	monitor.chartView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	monitor.logView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	// Get current time for initial display
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// Set initial content
	monitor.statsView.SetText(fmt.Sprintf("[white]WiFi Quality Monitor\n"+
		"Current Time: [cyan]%s[white]\n"+
		"Total Tests: 0 | [green]Success: 0[white] | [red]Failure: 0[white]\n"+
		"Success Rate: [yellow]0.00%%[white]\n", currentTime))

	monitor.chartView.SetText("Test Results:\n\n" +
		"[yellow]DHCP Test Results (Every 5 minutes):[white]\n" +
		"  [yellow]Waiting for first DHCP test...[white]\n\n" +
		"[yellow]Ping Test Results (Every 1 minute):[white]\n" +
		"  [yellow]Waiting for first ping test...[white]")

	monitor.logView.SetText("Latest Test Results:\n\n" +
		"[yellow]Latest DHCP Test:[white]\n" +
		"[yellow]No DHCP tests completed yet.[white]\n\n" +
		"[yellow]Latest Ping Test:[white]\n" +
		"[yellow]No ping tests completed yet.[white]")

	// Create layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(monitor.statsView, 5, 1, false).
		AddItem(monitor.chartView, 0, 2, false).
		AddItem(monitor.logView, 15, 1, true)

	// Start monitoring
	go monitor.startMonitoring()

	// Run application
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
