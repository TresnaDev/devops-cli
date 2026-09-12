package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"devops-cli/internal/crypto"
	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func sendTelegramAlert(serverName, host, errorMsg string) {
	token := database.GetSetting("telegram_token")
	chatID := database.GetSetting("telegram_chat_id")

	if token == "" || chatID == "" {
		return // Not configured
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	message := fmt.Sprintf(
		"🚨 <b>SERVER DOWN DETECTED</b>\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🖥  <b>Server:</b> <code>%s</code>\n"+
			"🌐  <b>Host:</b> <code>%s</code>\n"+
			"❌  <b>Status:</b> <code>DOWN</code>\n"+
			"⚠️  <b>Reason:</b> %s\n"+
			"🕐  <b>Detected at:</b> <code>%s</code>\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🔧 Segera periksa koneksi atau status server.\n\n"+
			"<i>— DevOps CLI Monitor</i>",
		serverName, host, errorMsg,
		time.Now().Format("02 Jan 2006, 15:04:05 WIB"),
	)

	payload := map[string]string{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err == nil {
		resp.Body.Close()
	}
}

var checkTargetID int

var checkTcpCmd = &cobra.Command{
	Use:   "check-tcp",
	Short: "Run TCP health check on all active servers (or single with --id)",
	Run: func(cmd *cobra.Command, args []string) {
		var servers []models.Server

		if checkTargetID != 0 {
			// Single server mode
			var s models.Server
			if err := database.DB.First(&s, checkTargetID).Error; err != nil {
				pterm.Error.Printf("Server with ID %d not found.\n", checkTargetID)
				return
			}
			if !s.IsActive {
				pterm.Warning.Printf("Server '%s' (ID: %d) is INACTIVE.\n", s.Name, checkTargetID)
				return
			}
			servers = []models.Server{s}
		} else {
			// All active servers
			if err := database.DB.Where("is_active = ?", true).Find(&servers).Error; err != nil {
				pterm.Error.Println("Failed to fetch servers:", err)
				return
			}
		}

		if len(servers) == 0 {
			pterm.Warning.Println("No active servers found. Add one with 'server-add'.")
			return
		}

		pterm.Info.Printf("Starting health check for %d server(s)...\n\n", len(servers))

		tableData := pterm.TableData{
			{"ID", "SERVER", "HOST", "PORT", "STATUS", "LATENCY"},
		}

		for i := range servers {
			s := &servers[i] // Reference to update DB
			target := fmt.Sprintf("%s:%d", s.Host, s.Port)

			spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Checking %s (%s)...", s.Name, target))

			start := time.Now()
			// TCP Ping to check if the port is open and responding
			conn, err := net.DialTimeout("tcp", target, 5*time.Second)
			latency := time.Since(start)

			if err != nil {
				spinner.Fail(fmt.Sprintf("%s is DOWN", s.Name))
				
				// Always log for audit trail
				database.DB.Create(&models.ServerEvent{
					ServerID:  s.ID,
					EventType: "TCP",
					Status:    "DOWN",
					Message:   err.Error(),
				})

				// Only send Telegram alert if status CHANGED
				if s.LastStatus != "DOWN" {
					sendTelegramAlert(s.Name, s.Host, err.Error())
				}
				s.LastStatus = "DOWN"
			} else {
				conn.Close()
				spinner.Success(fmt.Sprintf("%s is UP (%v)", s.Name, latency.Round(time.Millisecond)))
				
				// Always log for audit trail
				database.DB.Create(&models.ServerEvent{
					ServerID:  s.ID,
					EventType: "TCP",
					Status:    "UP",
					Message:   fmt.Sprintf("Connection successful (%v)", latency.Round(time.Millisecond)),
				})
				
				s.LastStatus = "UP"
			}

			// Update LastStatus in Database
			database.DB.Save(s)

			// Format for final table
			statusStr := pterm.FgGreen.Sprint("UP")
			if s.LastStatus == "DOWN" {
				statusStr = pterm.FgRed.Sprint("DOWN")
			}

			latencyStr := latency.Round(time.Millisecond).String()
			if s.LastStatus == "DOWN" {
				latencyStr = pterm.FgRed.Sprint("TIMEOUT")
			}

			tableData = append(tableData, []string{
				fmt.Sprintf("%d", s.ID),
				s.Name,
				s.Host,
				fmt.Sprintf("%d", s.Port),
				statusStr,
				latencyStr,
			})
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()

		pterm.Success.Println("\nHealth check completed! Database has been updated.")
	},
}

var checkHistoryCmd = &cobra.Command{
	Use:   "check-history",
	Short: "View health and credential check history of servers",
	Run: func(cmd *cobra.Command, args []string) {
		var servers []models.Server
		if err := database.DB.Find(&servers).Error; err != nil {
			pterm.Error.Println("Failed to fetch servers:", err)
			return
		}

		if len(servers) == 0 {
			pterm.Warning.Println("No servers found.")
			return
		}

		tableData := pterm.TableData{
			{"ID", "SERVER", "HOST", "TCP STATUS", "TCP CHECKED", "CREDENTIAL", "CRED STATUS", "CRED CHECKED"},
		}

		for _, s := range servers {
			// TCP Status coloring
			tcpStatusStr := pterm.FgGray.Sprint(s.LastStatus)
			if s.LastStatus == "UP" {
				tcpStatusStr = pterm.FgGreen.Sprint("UP ✓")
			} else if s.LastStatus == "DOWN" {
				tcpStatusStr = pterm.FgRed.Sprint("DOWN ✗")
			} else if s.LastStatus == "" {
				tcpStatusStr = pterm.FgGray.Sprint("UNCHECKED")
			}

			// Last checked time
			tcpLastCheckedStr := "-"
			if !s.UpdatedAt.IsZero() && s.LastStatus != "" {
				tcpLastCheckedStr = s.UpdatedAt.Format("02 Jan 06 15:04")
			}

			// Fetch credentials for this server
			var creds []models.ServerCredential
			database.DB.Where("server_id = ?", s.ID).Find(&creds)

			if len(creds) == 0 {
				// No credentials, just print the server row
				tableData = append(tableData, []string{
					fmt.Sprintf("%d", s.ID),
					s.Name,
					s.Host,
					tcpStatusStr,
					tcpLastCheckedStr,
					pterm.FgGray.Sprint("(none)"),
					pterm.FgGray.Sprint("-"),
					"-",
				})
				continue
			}

			// Print a row for each credential
			for i, c := range creds {
				// Only show server ID, Name, Host, TCP on the first row for cleaner look,
				// or just repeat them. Repeating is better for grepping/filtering.
				
				displayID := fmt.Sprintf("%d", s.ID)
				displayName := s.Name
				displayHost := s.Host
				displayTcpStatus := tcpStatusStr
				displayTcpCheck := tcpLastCheckedStr

				// If you want to group them visually by leaving blanks for duplicates:
				if i > 0 {
					displayID = ""
					displayName = ""
					displayHost = ""
					displayTcpStatus = ""
					displayTcpCheck = ""
				}

				credStatusStr := pterm.FgGray.Sprint("unchecked")
				switch c.Status {
				case "valid":
					credStatusStr = pterm.FgGreen.Sprint("VALID ✓")
				case "invalid":
					credStatusStr = pterm.FgRed.Sprint("INVALID ✗")
				}

				credLastCheckedStr := "-"
				if c.LastChecked != nil {
					credLastCheckedStr = c.LastChecked.Format("02 Jan 06 15:04")
				}

				tableData = append(tableData, []string{
					displayID,
					displayName,
					displayHost,
					displayTcpStatus,
					displayTcpCheck,
					c.Label,
					credStatusStr,
					credLastCheckedStr,
				})
			}
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()
		fmt.Println()
	},
}

var monitorLatencyCmd = &cobra.Command{
	Use:   "monitor-latency",
	Short: "Monitor server latency in real-time",
	Run: func(cmd *cobra.Command, args []string) {
		var servers []models.Server
		if err := database.DB.Where("is_active = ?", true).Find(&servers).Error; err != nil {
			pterm.Error.Println("Failed to fetch servers:", err)
			return
		}

		if len(servers) == 0 {
			pterm.Warning.Println("No active servers found.")
			return
		}

		pterm.Info.Println("Starting real-time latency monitor. Press Ctrl+C to stop.")

		area, _ := pterm.DefaultArea.Start()
		defer area.Stop()

		stopChan := make(chan struct{})
		
		go func() {
			// Put terminal into raw mode to catch single keystrokes
			oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				// If we can't make it raw, just fallback and close
				close(stopChan)
				return
			}
			defer term.Restore(int(os.Stdin.Fd()), oldState)

			b := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(b)
				if err != nil || n == 0 {
					continue
				}
				// 3 = Ctrl+C, 4 = Ctrl+D, 27 = ESC, 113 = 'q', 81 = 'Q'
				if b[0] == 3 || b[0] == 4 || b[0] == 27 || b[0] == 'q' || b[0] == 'Q' {
					close(stopChan)
					return
				}
			}
		}()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Keep track of average latency
		avgLatencies := make(map[uint][]time.Duration)
		
		// Force first tick immediately
		firstTick := make(chan bool, 1)
		firstTick <- true

		for {
			select {
			case <-stopChan:
				area.Stop()
				fmt.Println()
				pterm.Info.Println("Monitoring stopped.")
				return
			case <-firstTick:
				// Fallthrough to tick logic
			case <-ticker.C:
				// Tick logic
			}

			var wg sync.WaitGroup
			var mu sync.Mutex

			type resultStruct struct {
				status  string
				latency time.Duration
			}
			results := make(map[uint]resultStruct)

			for _, s := range servers {
				wg.Add(1)
				go func(srv models.Server) {
					defer wg.Done()
					target := fmt.Sprintf("%s:%d", srv.Host, srv.Port)
					start := time.Now()
					conn, err := net.DialTimeout("tcp", target, 2*time.Second)
					latency := time.Since(start)

					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						results[srv.ID] = resultStruct{"DOWN", 0}
					} else {
						conn.Close()
						results[srv.ID] = resultStruct{"UP", latency}
					}
				}(s)
			}
			wg.Wait()

			tableData := pterm.TableData{
				{"SERVER", "HOST", "STATUS", "LATENCY", "AVG (Last 10)"},
			}

			for _, s := range servers {
				res := results[s.ID]

				// Update history
				history := avgLatencies[s.ID]
				if res.status == "UP" {
					history = append(history, res.latency)
				} else {
					// 0 means timeout for average calculation
					history = append(history, 2*time.Second) 
				}
				if len(history) > 10 {
					history = history[1:]
				}
				avgLatencies[s.ID] = history

				// Calculate average
				var total time.Duration
				for _, l := range history {
					total += l
				}
				avgLat := total / time.Duration(len(history))

				// Formatting
				statusStr := pterm.FgRed.Sprint("DOWN ✗")
				latStr := pterm.FgRed.Sprint("TIMEOUT")
				avgStr := pterm.FgRed.Sprint("TIMEOUT")

				if res.status == "UP" {
					statusStr = pterm.FgGreen.Sprint("UP ✓")
					
					// Color code current latency
					latMs := res.latency.Milliseconds()
					if latMs < 50 {
						latStr = pterm.FgGreen.Sprintf("%v", res.latency.Round(time.Millisecond))
					} else if latMs < 150 {
						latStr = pterm.FgYellow.Sprintf("%v", res.latency.Round(time.Millisecond))
					} else {
						latStr = pterm.FgRed.Sprintf("%v", res.latency.Round(time.Millisecond))
					}

					// Color code average latency
					avgMs := avgLat.Milliseconds()
					if avgMs < 50 {
						avgStr = pterm.FgGreen.Sprintf("%v", avgLat.Round(time.Millisecond))
					} else if avgMs < 150 {
						avgStr = pterm.FgYellow.Sprintf("%v", avgLat.Round(time.Millisecond))
					} else {
						avgStr = pterm.FgRed.Sprintf("%v", avgLat.Round(time.Millisecond))
					}
				} else {
					// Even if DOWN, if average is not purely timeout, show it
					if avgLat < 2*time.Second {
						avgStr = pterm.FgYellow.Sprintf("%v", avgLat.Round(time.Millisecond))
					}
				}

				tableData = append(tableData, []string{
					s.Name,
					s.Host,
					statusStr,
					latStr,
					avgStr,
				})
			}

			tableStr, _ := pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
				WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
				WithData(tableData).Srender()

			currentTime := time.Now().Format("15:04:05")
			header := pterm.FgCyan.Sprintf("Live Latency Monitor (Updated: %s)\n", currentTime)
			
			area.Update(header + tableStr)
		}
	},
}

func sendTelegramCredentialAlert(serverName, host, user, reason string) {
	token := database.GetSetting("telegram_token")
	chatID := database.GetSetting("telegram_chat_id")

	if token == "" || chatID == "" {
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	message := fmt.Sprintf(
		"🔐 <b>CREDENTIAL INVALID DETECTED</b>\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🖥  <b>Server:</b> <code>%s</code>\n"+
			"🌐  <b>Host:</b> <code>%s</code>\n"+
			"👤  <b>User:</b> <code>%s</code>\n"+
			"❌  <b>Status:</b> <code>INVALID</code>\n"+
			"⚠️  <b>Reason:</b> %s\n"+
			"🕐  <b>Detected at:</b> <code>%s</code>\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛡 Kemungkinan password telah diubah atau akun dinonaktifkan.\n"+
			"Segera verifikasi credential dan perbarui dengan <code>server-edit</code>.\n\n"+
			"<i>— DevOps CLI Monitor</i>",
		serverName, host, user, reason,
		time.Now().Format("02 Jan 2006, 15:04:05 WIB"),
	)

	payload := map[string]string{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err == nil {
		resp.Body.Close()
	}
}

// trySSHAuth attempts to authenticate to an SSH server and returns an error if it fails.
// It tries password auth first, then key-based auth, depending on what's configured.
func trySSHAuth(s *models.Server) (string, error) {
	var authMethods []gossh.AuthMethod

	// Method 1: Password auth
	if s.SSHPassword != "" {
		password, err := crypto.Decrypt(s.SSHPassword)
		if err != nil {
			return "password", fmt.Errorf("failed to decrypt stored password")
		}
		authMethods = append(authMethods, gossh.Password(password))
	}

	// Method 2: SSH Key auth
	if s.SSHKeyPath != "" {
		keyPath := s.SSHKeyPath
		// Expand ~ to home dir
		if len(keyPath) > 1 && keyPath[:2] == "~/" {
			home, _ := os.UserHomeDir()
			keyPath = home + keyPath[1:]
		}
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return "key", fmt.Errorf("cannot read key file: %s", keyPath)
		}
		signer, err := gossh.ParsePrivateKey(keyBytes)
		if err != nil {
			return "key", fmt.Errorf("invalid private key file: %v", err)
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return "none", fmt.Errorf("no credentials configured")
	}

	config := &gossh.ClientConfig{
		User:            s.User,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // We trust the host from the DB
		Timeout:         8 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	client, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		// Determine which method was attempted for better error reporting
		method := "password"
		if s.SSHPassword == "" && s.SSHKeyPath != "" {
			method = "key"
		}
		return method, err
	}
	client.Close()
	return "", nil
}

var checkCredTargetID int

var checkCredentialCmd = &cobra.Command{
	Use:   "check-credential",
	Short: "Validate all SSH credentials for servers (or single server with --id)",
	Run: func(cmd *cobra.Command, args []string) {
		var servers []models.Server

		if checkCredTargetID != 0 {
			var s models.Server
			if err := database.DB.First(&s, checkCredTargetID).Error; err != nil {
				pterm.Error.Printf("Server with ID %d not found.\n", checkCredTargetID)
				return
			}
			if !s.IsActive {
				pterm.Warning.Printf("Server '%s' (ID: %d) is INACTIVE.\n", s.Name, checkCredTargetID)
				return
			}
			servers = []models.Server{s}
		} else {
			if err := database.DB.Where("is_active = ?", true).Find(&servers).Error; err != nil {
				pterm.Error.Println("Failed to fetch servers:", err)
				return
			}
		}

		if len(servers) == 0 {
			pterm.Warning.Println("No active servers found.")
			return
		}

		tableData := pterm.TableData{
			{"SERVER ID", "SERVER", "CRED ID", "LABEL", "USER", "AUTH", "STATUS", "LAST CHECKED"},
		}

		now := time.Now()
		totalChecked := 0

		for _, s := range servers {
			var creds []models.ServerCredential
			database.DB.Where("server_id = ?", s.ID).Find(&creds)

			if len(creds) == 0 {
				pterm.Warning.Printf("Server '%s' has no credentials. Use 'cred-add --server-id %d'.\n", s.Name, s.ID)
				continue
			}

			for i := range creds {
				c := &creds[i]
				totalChecked++

				authMethod := "password"
				if c.SSHPassword == "" && c.SSHKeyPath != "" {
					authMethod = "key"
				} else if c.SSHPassword != "" && c.SSHKeyPath != "" {
					authMethod = "password+key"
				}

				spinner, _ := pterm.DefaultSpinner.Start(
					fmt.Sprintf("Authenticating [%s] %s@%s...", c.Label, c.User, s.Host))

				err := trySSHCredential(c, s.Host, s.Port)

				var credStatusStr string
				if err != nil {
					spinner.Fail(fmt.Sprintf("[%s] - %s@%s — INVALID: %v", s.Name, c.User, s.Host, err))

					// Always log for audit trail
					database.DB.Create(&models.ServerEvent{
						ServerID:  s.ID,
						EventType: "CREDENTIAL",
						Status:    "INVALID",
						Message:   fmt.Sprintf("[%s] %s: %s", c.Label, c.User, err.Error()),
					})

					// Alert only if status changed
					if c.Status != "invalid" {
						sendTelegramCredentialAlert(s.Name, s.Host, c.User, fmt.Sprintf("[%s] %s", c.Label, err.Error()))
					}
					c.Status = "invalid"
					credStatusStr = pterm.FgRed.Sprint("INVALID ✗")
				} else {
					spinner.Success(fmt.Sprintf("[%s] - %s@%s — OK", s.Name, c.User, s.Host))

					// Always log for audit trail
					database.DB.Create(&models.ServerEvent{
						ServerID:  s.ID,
						EventType: "CREDENTIAL",
						Status:    "VALID",
						Message:   fmt.Sprintf("[%s] %s: Authentication successful", c.Label, c.User),
					})

					c.Status = "valid"
					credStatusStr = pterm.FgGreen.Sprint("VALID ✓")
				}

				c.LastChecked = &now
				if err := database.DB.Save(c).Error; err != nil { pterm.Error.Println("SAVE ERROR:", err) }

				displayServerID := fmt.Sprintf("%d", s.ID)
				displayServerName := s.Name
				if i > 0 {
					displayServerID = ""
					displayServerName = ""
				}

				tableData = append(tableData, []string{
					displayServerID,
					displayServerName,
					fmt.Sprintf("%d", c.ID),
					c.Label,
					c.User,
					authMethod,
					credStatusStr,
					now.Format("02 Jan 15:04:05"),
				})
			}
		}

		if totalChecked == 0 {
			pterm.Warning.Println("No credentials found to check. Add credentials with 'cred-add'.")
			return
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()

		pterm.Success.Printf("\nCredential check completed! Checked %d credential(s).\n", totalChecked)
	},
}

func init() {
	checkTcpCmd.Flags().IntVar(&checkTargetID, "id", 0, "Check a specific server by ID (default: all active servers)")
	checkCredentialCmd.Flags().IntVar(&checkCredTargetID, "id", 0, "Check credential for a specific server by ID (default: all)")

	rootCmd.AddCommand(checkTcpCmd)
	rootCmd.AddCommand(checkHistoryCmd)
	rootCmd.AddCommand(checkCredentialCmd)
	rootCmd.AddCommand(monitorLatencyCmd)
}
