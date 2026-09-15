package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// ── Telegram Alert for HTTP endpoints ────────────────────────────────────────

func sendTelegramEndpointBatchedAlert(entries []alertEntry) {
	token := database.GetSetting("telegram_token")
	chatID := database.GetSetting("telegram_chat_id")
	if token == "" || chatID == "" {
		return
	}

	lines := ""
	for _, e := range entries {
		switch e.status {
		case "DOWN":
			lines += fmt.Sprintf("%s  <b>%s</b>  →  <code>DOWN</code>\n    <i>%s</i>\n", e.icon, e.serverName, e.reason)
		case "RESOLVED":
			lines += fmt.Sprintf("%s  <b>%s</b>  →  <code>RESOLVED</code>\n    <i>%s</i>\n", e.icon, e.serverName, e.reason)
		}
	}

	message := fmt.Sprintf(
		"🌐 <b>ENDPOINT STATUS UPDATE</b>\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"%s"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🕐 <b>Checked at:</b> <code>%s</code>\n\n"+
			"<i>— DevOps CLI Monitor</i>",
		lines,
		time.Now().Format("02 Jan 2006, 15:04:05 WIB"),
	)

	payload := map[string]string{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token), "application/json", bytes.NewBuffer(jsonPayload))
	if err == nil {
		resp.Body.Close()
	}
}

// ── endpoint-list ─────────────────────────────────────────────────────────────

var endpointListCmd = &cobra.Command{
	Use:   "endpoint-list",
	Short: "List all monitored HTTP/HTTPS endpoints",
	Run: func(cmd *cobra.Command, args []string) {
		var endpoints []models.Endpoint
		if err := database.DB.Find(&endpoints).Error; err != nil {
			pterm.Error.Println("Failed to fetch endpoints:", err)
			return
		}

		if len(endpoints) == 0 {
			pterm.Info.Println("No endpoints registered. Add one with 'endpoint-add'.")
			return
		}

		tableData := pterm.TableData{
			{"ID", "NAME", "URL", "EXPECT", "TIMEOUT", "ACTIVE", "STATUS", "LAST CHECKED"},
		}

		for _, e := range endpoints {
			statusStr := pterm.FgGray.Sprint(e.LastStatus)
			switch e.LastStatus {
			case "UP":
				statusStr = pterm.FgGreen.Sprint("UP ✓")
			case "DOWN":
				statusStr = pterm.FgRed.Sprint("DOWN ✗")
			}

			activeStr := pterm.FgGreen.Sprint("YES")
			if !e.IsActive {
				activeStr = pterm.FgRed.Sprint("NO")
			}

			lastChecked := "never"
			if e.LastChecked != nil {
				lastChecked = e.LastChecked.Format("02 Jan 15:04:05")
			}

			keyword := e.Keyword
			if keyword == "" {
				keyword = "-"
			}

			tableData = append(tableData, []string{
				fmt.Sprintf("%d", e.ID),
				e.Name,
				e.URL,
				fmt.Sprintf("%d / %s", e.ExpectedStatus, keyword),
				fmt.Sprintf("%ds", e.TimeoutSeconds),
				activeStr,
				statusStr,
				lastChecked,
			})
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()
		fmt.Println()
	},
}

// ── endpoint-add ──────────────────────────────────────────────────────────────

var endpointAddCmd = &cobra.Command{
	Use:   "endpoint-add",
	Short: "Add an HTTP/HTTPS endpoint to monitor (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Add Endpoint")

		name, cancelled := ask("Name / Label")
		if cancelled {
			return
		}
		if name == "" {
			pterm.Error.Println("Name is required.")
			return
		}

		pterm.FgGray.Println("  Example: https://example.com or https://api.example.com/health")
		url, cancelled := ask("URL")
		if cancelled {
			return
		}
		if url == "" {
			pterm.Error.Println("URL is required.")
			return
		}

		expectedStatus, cancelled := askIntDefault("Expected HTTP Status Code", 200)
		if cancelled {
			return
		}

		pterm.FgGray.Println("  Optional: keyword that must appear in the response body (leave empty to skip)")
		keyword, cancelled := askOptional("Keyword in response body")
		if cancelled {
			return
		}

		timeoutSec, cancelled := askIntDefault("Timeout (seconds)", 10)
		if cancelled {
			return
		}

		fmt.Println()
		endpoint := models.Endpoint{
			Name:           name,
			URL:            url,
			ExpectedStatus: expectedStatus,
			Keyword:        keyword,
			TimeoutSeconds: timeoutSec,
			IsActive:       true,
			LastStatus:     "unknown",
		}

		if err := database.DB.Create(&endpoint).Error; err != nil {
			pterm.Error.Println("Failed to save endpoint:", err)
			return
		}

		pterm.Success.Printf("Endpoint '%s' added with ID %d.\n", endpoint.Name, endpoint.ID)
		pterm.Info.Println("Run 'check-http' to perform the first health check.")
	},
}

// ── endpoint-remove ───────────────────────────────────────────────────────────

var endpointRemoveCmd = &cobra.Command{
	Use:   "endpoint-remove",
	Short: "Remove a monitored endpoint (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Remove Endpoint")

		id, cancelled := askInt("Endpoint ID")
		if cancelled {
			return
		}
		if id == 0 {
			pterm.Error.Println("Valid Endpoint ID is required.")
			return
		}

		var endpoint models.Endpoint
		if err := database.DB.First(&endpoint, id).Error; err != nil {
			pterm.Error.Printf("Endpoint with ID %d not found.\n", id)
			return
		}

		confirmed, cancelled := askConfirm(fmt.Sprintf("Delete endpoint '%s' (%s)?", endpoint.Name, endpoint.URL))
		if cancelled || !confirmed {
			pterm.Info.Println("Cancelled.")
			return
		}

		database.DB.Delete(&endpoint)
		fmt.Println()
		pterm.Success.Printf("Endpoint '%s' removed.\n", endpoint.Name)
	},
}

// ── endpoint-toggle ───────────────────────────────────────────────────────────

var endpointToggleCmd = &cobra.Command{
	Use:   "endpoint-toggle",
	Short: "Toggle an endpoint active/paused (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Toggle Endpoint")

		id, cancelled := askInt("Endpoint ID")
		if cancelled {
			return
		}

		var endpoint models.Endpoint
		if err := database.DB.First(&endpoint, id).Error; err != nil {
			pterm.Error.Printf("Endpoint with ID %d not found.\n", id)
			return
		}

		endpoint.IsActive = !endpoint.IsActive
		database.DB.Save(&endpoint)

		fmt.Println()
		if endpoint.IsActive {
			pterm.Success.Printf("Endpoint '%s' is now ACTIVE.\n", endpoint.Name)
		} else {
			pterm.Warning.Printf("Endpoint '%s' is now PAUSED.\n", endpoint.Name)
		}
	},
}

// ── check-http ────────────────────────────────────────────────────────────────

var checkHTTPTargetID int

var checkHTTPCmd = &cobra.Command{
	Use:   "check-http",
	Short: "Run HTTP health check on all active endpoints (or single with --id)",
	Run: func(cmd *cobra.Command, args []string) {
		var endpoints []models.Endpoint

		if checkHTTPTargetID != 0 {
			var e models.Endpoint
			if err := database.DB.First(&e, checkHTTPTargetID).Error; err != nil {
				pterm.Error.Printf("Endpoint with ID %d not found.\n", checkHTTPTargetID)
				return
			}
			endpoints = []models.Endpoint{e}
		} else {
			if err := database.DB.Where("is_active = ?", true).Find(&endpoints).Error; err != nil {
				pterm.Error.Println("Failed to fetch endpoints:", err)
				return
			}
		}

		if len(endpoints) == 0 {
			pterm.Warning.Println("No active endpoints found. Add one with 'endpoint-add'.")
			return
		}

		pterm.Info.Printf("Starting HTTP check for %d endpoint(s)...\n\n", len(endpoints))

		tableData := pterm.TableData{
			{"ID", "NAME", "URL", "STATUS CODE", "STATUS", "LATENCY"},
		}

		// Batched alerts collector
		var alertEntries []alertEntry
		now := time.Now()

		for i := range endpoints {
			e := &endpoints[i]

			spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Checking %s (%s)...", e.Name, e.URL))

			client := &http.Client{
				Timeout: time.Duration(e.TimeoutSeconds) * time.Second,
				// Follow redirects but limit to 5
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if len(via) >= 5 {
						return fmt.Errorf("too many redirects")
					}
					return nil
				},
			}

			start := time.Now()
			resp, err := client.Get(e.URL)
			latency := time.Since(start)

			prevStatus := e.LastStatus
			e.LastChecked = &now

			var statusCodeStr, statusStr, latencyStr string
			isDown := false
			downReason := ""

			if err != nil {
				isDown = true
				downReason = err.Error()
				statusCodeStr = pterm.FgRed.Sprint("ERR")
				statusStr = pterm.FgRed.Sprint("DOWN ✗")
				latencyStr = pterm.FgRed.Sprint("TIMEOUT")
				spinner.Fail(fmt.Sprintf("%s — DOWN: %v", e.Name, err))
			} else {
				defer resp.Body.Close()
				statusCodeStr = fmt.Sprintf("%d", resp.StatusCode)
				latencyStr = latency.Round(time.Millisecond).String()

				// Check expected status code
				if resp.StatusCode != e.ExpectedStatus {
					isDown = true
					downReason = fmt.Sprintf("Expected %d, got %d", e.ExpectedStatus, resp.StatusCode)
					statusCodeStr = pterm.FgRed.Sprintf("%d", resp.StatusCode)
					statusStr = pterm.FgRed.Sprint("DOWN ✗")
					spinner.Fail(fmt.Sprintf("%s — Unexpected status: %d (expected %d)", e.Name, resp.StatusCode, e.ExpectedStatus))
				} else if e.Keyword != "" {
					// Check keyword in body
					bodyBytes, _ := io.ReadAll(resp.Body)
					if !strings.Contains(string(bodyBytes), e.Keyword) {
						isDown = true
						downReason = fmt.Sprintf("Keyword '%s' not found in response", e.Keyword)
						statusCodeStr = pterm.FgYellow.Sprintf("%d", resp.StatusCode)
						statusStr = pterm.FgRed.Sprint("DOWN ✗")
						spinner.Fail(fmt.Sprintf("%s — Keyword '%s' not found in body", e.Name, e.Keyword))
					}
				}

				if !isDown {
					statusCodeStr = pterm.FgGreen.Sprintf("%d", resp.StatusCode)
					statusStr = pterm.FgGreen.Sprint("UP ✓")
					spinner.Success(fmt.Sprintf("%s — UP %d (%v)", e.Name, resp.StatusCode, latency.Round(time.Millisecond)))
				}

				e.LastStatusCode = resp.StatusCode
			}

			if isDown {
				// Always log for audit trail
				database.DB.Create(&models.EndpointEvent{
					EndpointID: e.ID,
					Status:     "DOWN",
					Message:    downReason,
				})

				e.LastStatus = "DOWN"
				// Collect batched alert only if status changed
				if prevStatus != "DOWN" {
					alertEntries = append(alertEntries, alertEntry{
						icon:       "🔴",
						serverName: e.Name,
						host:       e.URL,
						status:     "DOWN",
						reason:     downReason,
					})
				}
			} else {
				// Always log for audit trail
				database.DB.Create(&models.EndpointEvent{
					EndpointID: e.ID,
					Status:     "UP",
					Message:    fmt.Sprintf("OK (%v)", latency.Round(time.Millisecond)),
				})

				e.LastStatus = "UP"
				// Collect resolved alert if previously down
				if prevStatus == "DOWN" {
					alertEntries = append(alertEntries, alertEntry{
						icon:       "✅",
						serverName: e.Name,
						host:       e.URL,
						status:     "RESOLVED",
						reason:     fmt.Sprintf("Back online (%v)", latency.Round(time.Millisecond)),
					})
				}
				if statusStr == "" {
					statusStr = pterm.FgGreen.Sprint("UP ✓")
				}
			}

			database.DB.Save(e)

			tableData = append(tableData, []string{
				fmt.Sprintf("%d", e.ID),
				e.Name,
				e.URL,
				statusCodeStr,
				statusStr,
				latencyStr,
			})
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()

		pterm.Success.Println("\nHTTP check completed!")

		// Send batched alert if any changes detected
		if len(alertEntries) > 0 {
			sendTelegramEndpointBatchedAlert(alertEntries)
		}
	},
}

var endpointHistoryID int
var endpointHistoryHours int
var endpointHistoryAll bool

var endpointHistoryCmd = &cobra.Command{
	Use:   "endpoint-history",
	Short: "View event history (DOWN/UP) for a specific endpoint",
	Run: func(cmd *cobra.Command, args []string) {
		if endpointHistoryID == 0 {
			pterm.Error.Println("Endpoint ID is required. Usage: endpoint-history --id <ID>")
			return
		}

		var e models.Endpoint
		if err := database.DB.First(&e, endpointHistoryID).Error; err != nil {
			pterm.Error.Printf("Endpoint with ID %d not found.\n", endpointHistoryID)
			return
		}

		var events []models.EndpointEvent
		query := database.DB.Where("endpoint_id = ?", endpointHistoryID)

		if !endpointHistoryAll {
			timeAgo := time.Now().Add(time.Duration(-endpointHistoryHours) * time.Hour)
			query = query.Where("created_at >= ?", timeAgo)
		}

		query.Order("created_at desc").Find(&events)

		if len(events) == 0 {
			if endpointHistoryAll {
				pterm.Info.Printf("No history events found for endpoint '%s'.\n", e.Name)
			} else {
				pterm.Info.Printf("No history events found for endpoint '%s' in the last %d hours.\n", e.Name, endpointHistoryHours)
			}
			return
		}

		tableData := pterm.TableData{
			{"TIMESTAMP", "STATUS", "MESSAGE"},
		}

		for _, ev := range events {
			statusStr := pterm.FgGray.Sprint(ev.Status)
			if ev.Status == "UP" {
				statusStr = pterm.FgGreen.Sprint(ev.Status)
			} else if ev.Status == "DOWN" {
				statusStr = pterm.FgRed.Sprint(ev.Status)
			}

			tableData = append(tableData, []string{
				ev.CreatedAt.Format("02 Jan 15:04:05"),
				statusStr,
				ev.Message,
			})
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()
		fmt.Println()
	},
}

func init() {
	checkHTTPCmd.Flags().IntVar(&checkHTTPTargetID, "id", 0, "Check a specific endpoint by ID (default: all active)")
	
	endpointHistoryCmd.Flags().IntVar(&endpointHistoryID, "id", 0, "Endpoint ID to view history")
	endpointHistoryCmd.Flags().IntVar(&endpointHistoryHours, "hours", 24, "View history for the last N hours")
	endpointHistoryCmd.Flags().BoolVar(&endpointHistoryAll, "all", false, "View all history (ignores --hours)")

	rootCmd.AddCommand(endpointListCmd)
	rootCmd.AddCommand(endpointAddCmd)
	rootCmd.AddCommand(endpointRemoveCmd)
	rootCmd.AddCommand(endpointToggleCmd)
	rootCmd.AddCommand(checkHTTPCmd)
	rootCmd.AddCommand(endpointHistoryCmd)
}
