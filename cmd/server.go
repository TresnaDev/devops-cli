package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"devops-cli/internal/crypto"
	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)


var serverListCmd = &cobra.Command{
	Use:   "server-list",
	Short: "List all configured servers",
	Run: func(cmd *cobra.Command, args []string) {
		var servers []models.Server
		if err := database.DB.Find(&servers).Error; err != nil {
			pterm.Error.Println("Failed to fetch servers:", err)
			return
		}

		if len(servers) == 0 {
			pterm.Info.Println("No servers configured. Try running 'server-add'.")
			return
		}

		tableData := pterm.TableData{
			{"ID", "NAME", "HOST", "PORT", "CREDENTIALS", "STATUS"},
		}

		for _, s := range servers {
			status := pterm.FgGreen.Sprint("ACTIVE")
			if !s.IsActive {
				status = pterm.FgRed.Sprint("INACTIVE")
			}

			// Count credentials from separate table
			var credCount int64
			database.DB.Model(&models.ServerCredential{}).Where("server_id = ?", s.ID).Count(&credCount)

			var credStr string
			if credCount == 0 {
				credStr = pterm.FgGray.Sprint("none")
			} else {
				credStr = fmt.Sprintf("%d configured", credCount)
			}

			tableData = append(tableData, []string{
				fmt.Sprintf("%d", s.ID),
				s.Name,
				s.Host,
				fmt.Sprintf("%d", s.Port),
				credStr,
				status,
			})
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).WithData(tableData).Render()
		pterm.FgGray.Println("\n  Use 'cred-list' to view SSH credentials per server.")
		fmt.Println()
	},
}

var serverAddCmd = &cobra.Command{
	Use:   "server-add",
	Short: "Add a new server to the database (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Add New Server")

		name, cancelled := ask("Server Name")
		if cancelled {
			return
		}
		if name == "" {
			pterm.Error.Println("Server Name is required.")
			return
		}

		host, cancelled := ask("Host / IP Address")
		if cancelled {
			return
		}
		if host == "" {
			pterm.Error.Println("Host is required.")
			return
		}

		user, cancelled := askDefault("SSH User", "root")
		if cancelled {
			return
		}

		port, cancelled := askIntDefault("SSH Port", 22)
		if cancelled {
			return
		}

		desc, cancelled := askOptional("Description")
		if cancelled {
			return
		}

		fmt.Println()
		newServer := models.Server{
			Name:        name,
			Host:        host,
			User:        user,
			Port:        port,
			Description: desc,
			IsActive:    true,
		}

		if err := database.DB.Create(&newServer).Error; err != nil {
			pterm.Error.Println("Failed to save server:", err)
			return
		}

		pterm.Success.Printf("Server '%s' (%s) added with ID: %d\n", name, host, newServer.ID)
		pterm.Info.Printf("Add SSH credentials with: cred-add  (Server ID: %d)\n", newServer.ID)
	},
}

var serverEditCmd = &cobra.Command{
	Use:   "server-edit",
	Short: "Edit an existing server (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Edit Server")

		idRaw, cancelled := askInt("Server ID")
		if cancelled {
			return
		}
		if idRaw == 0 {
			pterm.Error.Println("Valid Server ID is required.")
			return
		}

		var server models.Server
		if err := database.DB.First(&server, idRaw).Error; err != nil {
			pterm.Error.Printf("Server with ID %d not found.\n", idRaw)
			return
		}

		pterm.Info.Printf("Editing: %s (%s) — Press Enter to keep current value.\n\n", server.Name, server.Host)

		updated := false

		name, cancelled := askDefault("Server Name", server.Name)
		if cancelled {
			return
		}
		if name != server.Name {
			server.Name = name
			updated = true
		}

		host, cancelled := askDefault("Host / IP Address", server.Host)
		if cancelled {
			return
		}
		if host != server.Host {
			server.Host = host
			updated = true
		}

		user, cancelled := askDefault("SSH User", server.User)
		if cancelled {
			return
		}
		if user != server.User {
			server.User = user
			updated = true
		}

		port, cancelled := askIntDefault("SSH Port", server.Port)
		if cancelled {
			return
		}
		if port != server.Port {
			server.Port = port
			updated = true
		}

		fmt.Println()
		if !updated {
			pterm.Warning.Println("No changes made.")
			return
		}

		if err := database.DB.Save(&server).Error; err != nil {
			pterm.Error.Println("Failed to update server:", err)
			return
		}
		pterm.Success.Printf("Server '%s' (ID: %d) updated successfully!\n", server.Name, idRaw)
	},
}

var serverRemoveCmd = &cobra.Command{
	Use:   "server-remove",
	Short: "Remove a server from the database (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Remove Server")

		idRaw, cancelled := askInt("Server ID to remove")
		if cancelled {
			return
		}
		if idRaw == 0 {
			pterm.Error.Println("Valid Server ID is required.")
			return
		}

		var server models.Server
		if err := database.DB.First(&server, idRaw).Error; err != nil {
			pterm.Error.Printf("Server with ID %d not found.\n", idRaw)
			return
		}

		fmt.Println()
		pterm.Warning.Printf("You are about to remove: %s (%s)\n", server.Name, server.Host)
		confirmed, cancelled := askConfirm("Are you sure you want to delete this server?")
		if cancelled || !confirmed {
			pterm.Info.Println("Cancelled.")
			return
		}

		if err := database.DB.Delete(&models.Server{}, idRaw).Error; err != nil {
			pterm.Error.Println("Failed to delete server:", err)
			return
		}

		pterm.Success.Printf("Server '%s' (ID: %d) has been removed.\n", server.Name, idRaw)
	},
}



var serverImportCmd = &cobra.Command{
	Use:   "server-import",
	Short: "Bulk add servers from a CSV file",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			pterm.Error.Println("Please provide a CSV file path. Usage: server-import <path-to-csv>")
			return
		}

		filePath := args[0]
		file, err := os.Open(filePath)
		if err != nil {
			pterm.Error.Printf("Failed to open file: %v\n", err)
			return
		}
		defer file.Close()

		reader := csv.NewReader(file)
		// Try to read header
		records, err := reader.ReadAll()
		if err != nil {
			pterm.Error.Printf("Failed to parse CSV: %v\n", err)
			return
		}

		if len(records) < 2 {
			pterm.Warning.Println("CSV file is empty or only contains a header.")
			return
		}

		// Assume first row is header
		headers := records[0]
		
		// Map header index
		headerMap := make(map[string]int)
		for i, h := range headers {
			headerMap[strings.TrimSpace(strings.ToLower(h))] = i
		}

		// Check basic headers
		if _, ok := headerMap["name"]; !ok {
			pterm.Error.Println("CSV must contain a 'name' column.")
			return
		}
		if _, ok := headerMap["host"]; !ok {
			pterm.Error.Println("CSV must contain a 'host' column.")
			return
		}

		successCount := 0
		failCount := 0

		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Importing servers from %s...", filePath))

		for i := 1; i < len(records); i++ {
			row := records[i]
			
			// Helper to get column safely
			getCol := func(colName string) string {
				idx, exists := headerMap[colName]
				if exists && idx < len(row) {
					return strings.TrimSpace(row[idx])
				}
				return ""
			}

			name := getCol("name")
			host := getCol("host")
			
			if name == "" || host == "" {
				failCount++
				continue
			}

			user := getCol("user")
			if user == "" {
				user = "root"
			}

			portStr := getCol("port")
			port := 22
			if portStr != "" {
				if p, err := strconv.Atoi(portStr); err == nil {
					port = p
				}
			}

			password := getCol("password")
			keyPath := getCol("key_path")

			newServer := models.Server{
				Name:       name,
				Host:       host,
				User:       user,
				Port:       port,
				SSHKeyPath: keyPath,
				IsActive:   true,
			}

			if password != "" {
				encrypted, err := crypto.Encrypt(password)
				if err == nil {
					newServer.SSHPassword = encrypted
				}
			}

			if err := database.DB.Create(&newServer).Error; err != nil {
				failCount++
			} else {
				successCount++
			}
		}

		spinner.Success(fmt.Sprintf("Import completed: %d added, %d failed.", successCount, failCount))
	},
}

func init() {
	// server-history still uses flags for non-interactive mode
	serverHistoryCmd.Flags().IntVarP(&serverHistoryID, "id", "x", 0, "Server ID (required)")
	serverHistoryCmd.Flags().IntVar(&historyHours, "hours", 12, "Filter events by last N hours (default 12)")
	serverHistoryCmd.Flags().BoolVar(&historyAll, "all", false, "Show all history regardless of time")
	serverHistoryCmd.Flags().StringVar(&historyExport, "export", "", "Export full history to CSV file path")

	rootCmd.AddCommand(serverListCmd)
	rootCmd.AddCommand(serverAddCmd)
	rootCmd.AddCommand(serverEditCmd)
	rootCmd.AddCommand(serverRemoveCmd)
	rootCmd.AddCommand(serverImportCmd)
	rootCmd.AddCommand(serverHistoryCmd)
}

var (
	serverHistoryID int
	historyHours    int
	historyAll      bool
	historyExport   string
)

var serverHistoryCmd = &cobra.Command{
	Use:   "server-history",
	Short: "View event history (DOWN/UP/INVALID) for a specific server",
	Run: func(cmd *cobra.Command, args []string) {
		if serverHistoryID == 0 {
			pterm.Error.Println("Server ID is required. Usage: server-history --id <ID>")
			return
		}

		var s models.Server
		if err := database.DB.First(&s, serverHistoryID).Error; err != nil {
			pterm.Error.Printf("Server with ID %d not found.\n", serverHistoryID)
			return
		}

		var events []models.ServerEvent
		query := database.DB.Where("server_id = ?", serverHistoryID)

		if !historyAll {
			timeAgo := time.Now().Add(time.Duration(-historyHours) * time.Hour)
			query = query.Where("created_at >= ?", timeAgo)
		}

		query.Order("created_at desc").Find(&events)

		if len(events) == 0 {
			if historyAll {
				pterm.Info.Printf("No history events found for server '%s'.\n", s.Name)
			} else {
				pterm.Info.Printf("No history events found for server '%s' in the last %d hours.\n", s.Name, historyHours)
			}
			return
		}

		// Handle export
		if historyExport != "" {
			file, err := os.Create(historyExport)
			if err != nil {
				pterm.Error.Printf("Failed to create export file: %v\n", err)
				return
			}
			defer file.Close()

			writer := csv.NewWriter(file)
			defer writer.Flush()

			writer.Write([]string{"TIMESTAMP", "TYPE", "STATUS", "MESSAGE"})
			for _, e := range events {
				writer.Write([]string{
					e.CreatedAt.Format("2006-01-02 15:04:05"),
					e.EventType,
					e.Status,
					e.Message,
				})
			}
			pterm.Success.Printf("Successfully exported %d events to %s\n", len(events), historyExport)
			return
		}

		if historyAll {
			pterm.Info.Printf("Showing ALL %d events for server: %s (%s)\n\n", len(events), s.Name, s.Host)
		} else {
			pterm.Info.Printf("Showing %d events (Last %d hours) for server: %s (%s)\n\n", len(events), historyHours, s.Name, s.Host)
		}

		tableData := pterm.TableData{
			{"TIMESTAMP", "TYPE", "STATUS", "MESSAGE"},
		}

		for _, e := range events {
			statusStr := e.Status
			if e.Status == "UP" || e.Status == "VALID" {
				statusStr = pterm.FgGreen.Sprint(e.Status)
			} else {
				statusStr = pterm.FgRed.Sprint(e.Status)
			}

			tableData = append(tableData, []string{
				e.CreatedAt.Format("02 Jan 2006 15:04:05"),
				e.EventType,
				statusStr,
				e.Message,
			})
		}

		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()
		fmt.Println()
	},
}
