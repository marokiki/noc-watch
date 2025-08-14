package main

import (
	"fmt"
	"os"
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
	logFile       string // Log file path for persistent storage
	headless      bool   // Run in headless mode (no TUI)
}

// NewWiFiMonitor creates a new WiFi monitor instance
func NewWiFiMonitor() *WiFiMonitor {
	// Get WiFi interface from environment variable, default to wlan0
	wifiInterface := os.Getenv("WIFI_INTERFACE")
	if wifiInterface == "" {
		wifiInterface = "wlan0"
	}

	// Get log file path from environment variable, default to current directory
	logFile := os.Getenv("LOG_FILE")
	if logFile == "" {
		logFile = "noc-watch.log"
	}

	// Check if running in headless mode
	headless := os.Getenv("HEADLESS") == "true"

	return &WiFiMonitor{
		dhcpTests:     make([]WiFiTest, 0),
		pingTests:     make([]WiFiTest, 0),
		wifiInterface: wifiInterface,
		logFile:       logFile,
		headless:      headless,
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
	if w.headless {
		return // No UI updates in headless mode
	}

	   // Calculate success rates
	   var successRate, dhcpSuccessRate, pingSuccessRate float64
	   if w.totalCount > 0 {
		   successRate = float64(w.successCount) / float64(w.totalCount) * 100
	   }
	   if len(w.dhcpTests) > 0 {
		   dhcpSuccesses := 0
		   for _, t := range w.dhcpTests {
			   if t.Success {
				   dhcpSuccesses++
			   }
		   }
		   dhcpSuccessRate = float64(dhcpSuccesses) / float64(len(w.dhcpTests)) * 100
	   }
	   if len(w.pingTests) > 0 {
		   pingSuccesses := 0
		   for _, t := range w.pingTests {
			   if t.Success {
				   pingSuccesses++
			   }
		   }
		   pingSuccessRate = float64(pingSuccesses) / float64(len(w.pingTests)) * 100
	   }

	   // Get current time
	   currentTime := time.Now().Format("2006-01-02 15:04:05")

	   // Update statistics display
	   statsText := fmt.Sprintf(
		   "[white]WiFi Quality Monitor - NOC Watch -\n"+
			   "Current Time: [cyan]%s[white]\n"+
			   "Total Tests: %d | [green]Success: %d[white] | [red]Failure: %d[white]\n"+
			   "Success Rate: [yellow]%.2f%%[white]\n"+
			   "DHCP Success Rate: [yellow]%.2f%%[white]\n"+
			   "Ping Success Rate: [yellow]%.2f%%[white]\n",
		   currentTime, w.totalCount, w.successCount, w.totalCount-w.successCount, successRate, dhcpSuccessRate, pingSuccessRate,
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
			status := "[red]x"
			if test.Success {
				status = "[green]o"
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
			status := "[red]x"
			if test.Success {
				status = "[green]o"
			}
			chartText += fmt.Sprintf("  [%d] %s IPv4: %v IPv6: %v Latency: %v\n",
				i+1, status, test.IPv4Connectivity, test.IPv6Connectivity, test.Latency)
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

// writeResultsToFile writes test results to a text file
func (w *WiFiMonitor) writeResultsToFile() error {
	file, err := os.OpenFile(w.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// Write summary
	_, err = fmt.Fprintf(file, "\n=== WiFi Quality Test Results - %s ===\n", currentTime)
	if err != nil {
		return err
	}

	// Write DHCP test results
	if len(w.dhcpTests) > 0 {
		latest := w.dhcpTests[len(w.dhcpTests)-1]
		_, err = fmt.Fprintf(file, "DHCP Test: Success=%v, Time=%v\n", latest.Success, latest.DHCPRenewTime)
		if err != nil {
			return err
		}
	}

	// Write ping test results
	if len(w.pingTests) > 0 {
		latest := w.pingTests[len(w.pingTests)-1]
		_, err = fmt.Fprintf(file, "Ping Test: Success=%v, IPv4=%v, IPv6=%v, Latency=%v\n",
			latest.Success, latest.IPv4Connectivity, latest.IPv6Connectivity, latest.Latency)
		if err != nil {
			return err
		}
	}

	   // Write statistics
	   var dhcpSuccessRate, pingSuccessRate float64
	   if len(w.dhcpTests) > 0 {
		   dhcpSuccesses := 0
		   for _, t := range w.dhcpTests {
			   if t.Success {
				   dhcpSuccesses++
			   }
		   }
		   dhcpSuccessRate = float64(dhcpSuccesses) / float64(len(w.dhcpTests)) * 100
	   }
	   if len(w.pingTests) > 0 {
		   pingSuccesses := 0
		   for _, t := range w.pingTests {
			   if t.Success {
				   pingSuccesses++
			   }
		   }
		   pingSuccessRate = float64(pingSuccesses) / float64(len(w.pingTests)) * 100
	   }
	   _, err = fmt.Fprintf(file, "Total Tests: %d, Success: %d, Success Rate: %.2f%%\n",
		   w.totalCount, w.successCount,
		   func() float64 {
			   if w.totalCount > 0 {
				   return float64(w.successCount) / float64(w.totalCount) * 100
			   }
			   return 0
		   }())
	   if err != nil {
		   return err
	   }
	   _, err = fmt.Fprintf(file, "DHCP Success Rate: %.2f%%\n", dhcpSuccessRate)
	   if err != nil {
		   return err
	   }
	   _, err = fmt.Fprintf(file, "Ping Success Rate: %.2f%%\n", pingSuccessRate)
	   if err != nil {
		   return err
	   }

	_, err = fmt.Fprintf(file, "==========================================\n")
	return err
}

// startMonitoring begins periodic WiFi quality testing
func (w *WiFiMonitor) startMonitoring() {
	if w.headless {
		// In headless mode, run tests and write results to file
		dhcpTicker := time.NewTicker(5 * time.Minute)
		defer dhcpTicker.Stop()

		pingTicker := time.NewTicker(1 * time.Minute)
		defer pingTicker.Stop()

		fileTicker := time.NewTicker(1 * time.Minute)
		defer fileTicker.Stop()

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

				w.updateUI() // Still update UI for consistency, but no TUI

			case <-pingTicker.C:
				// Run only connectivity and latency tests (skip DHCP)
				test := w.runConnectivityTest()
				w.pingTests = append(w.pingTests, test)
				w.totalCount++

				if test.Success {
					w.successCount++
				}

				w.updateUI() // Still update UI for consistency, but no TUI

			case <-fileTicker.C:
				// Write results to file every minute
				if err := w.writeResultsToFile(); err != nil {
					fmt.Printf("Error writing to file: %v\n", err)
				}
			}
		}
	} else {
		// In TUI mode, run tests and update UI
		dhcpTicker := time.NewTicker(5 * time.Minute)
		defer dhcpTicker.Stop()

		pingTicker := time.NewTicker(1 * time.Minute)
		defer pingTicker.Stop()

		uiTicker := time.NewTicker(1 * time.Second)
		defer uiTicker.Stop()

		fileTicker := time.NewTicker(1 * time.Minute)
		defer fileTicker.Stop()

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

			case <-fileTicker.C:
				// Write results to file every minute
				if err := w.writeResultsToFile(); err != nil {
					fmt.Printf("Error writing to file: %v\n", err)
				}
			}
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

	// Create TUI application if not headless
	if !monitor.headless {
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
	} else {
		// In headless mode, just start monitoring and write results
		monitor.startMonitoring()
	}
}
