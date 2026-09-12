package cmd

import (
	"fmt"
	"os/exec"
	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)


var cronListCmd = &cobra.Command{
	Use:   "cron-list",
	Short: "List all scheduled background jobs",
	Run: func(cmd *cobra.Command, args []string) {
		var jobs []models.Cronjob
		if err := database.DB.Find(&jobs).Error; err != nil {
			pterm.Error.Println("Failed to fetch cronjobs:", err)
			return
		}

		if len(jobs) == 0 {
			pterm.Info.Println("No cron jobs configured. Try adding one with 'cron-add'.")
			return
		}

		tableData := pterm.TableData{
			{"ID", "NAME", "SCHEDULE", "COMMAND", "STATUS", "LAST RUN"},
		}

		for _, j := range jobs {
			statusStr := pterm.FgGreen.Sprint("ACTIVE")
			if !j.IsActive {
				statusStr = pterm.FgRed.Sprint("PAUSED")
			}
			
			lastRunStr := "-"
			if !j.LastRun.IsZero() {
				lastRunStr = j.LastRun.Format("02 Jan 15:04:05")
			}

			tableData = append(tableData, []string{
				fmt.Sprintf("%d", j.ID),
				j.Name,
				j.Schedule,
				j.Command,
				statusStr,
				lastRunStr,
			})
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()
		fmt.Println()
	},
}

var cronAddCmd = &cobra.Command{
	Use:   "cron-add",
	Short: "Add a new background job (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Add Scheduled Job")

		name, cancelled := ask("Job Name")
		if cancelled {
			return
		}
		if name == "" {
			pterm.Error.Println("Job Name is required.")
			return
		}

		pterm.FgGray.Println("  Schedule examples: '*/5 * * * *' (every 5 min), '0 * * * *' (every hour)")
		schedule, cancelled := ask("Cron Schedule")
		if cancelled {
			return
		}
		if schedule == "" {
			pterm.Error.Println("Schedule is required.")
			return
		}

		pterm.FgGray.Println("  Command examples: 'devops check-tcp', 'devops check-credential'")
		command, cancelled := ask("Command to Execute")
		if cancelled {
			return
		}
		if command == "" {
			pterm.Error.Println("Command is required.")
			return
		}

		fmt.Println()
		newJob := models.Cronjob{
			Name:     name,
			Schedule: schedule,
			Command:  command,
			IsActive: true,
		}

		if err := database.DB.Create(&newJob).Error; err != nil {
			pterm.Error.Println("Failed to save cron job:", err)
			return
		}

		pterm.Success.Printf("Job '%s' scheduled successfully! (ID: %d)\n", name, newJob.ID)
		pterm.Warning.Println("Restart the daemon for the new job to take effect: sudo systemctl restart devops")
	},
}

var cronToggleCmd = &cobra.Command{
	Use:   "cron-toggle",
	Short: "Toggle a background job Active/Paused (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Toggle Cron Job")

		idRaw, cancelled := askInt("Cron Job ID to toggle")
		if cancelled {
			return
		}
		if idRaw == 0 {
			pterm.Error.Println("Valid Cron Job ID is required.")
			return
		}

		var job models.Cronjob
		if err := database.DB.First(&job, idRaw).Error; err != nil {
			pterm.Error.Printf("Cron job with ID %d not found.\n", idRaw)
			return
		}

		job.IsActive = !job.IsActive
		database.DB.Save(&job)

		status := pterm.FgRed.Sprint("PAUSED")
		if job.IsActive {
			status = pterm.FgGreen.Sprint("ACTIVE")
		}
		fmt.Println()
		pterm.Success.Printf("Job '%s' is now %s\n", job.Name, status)
	},
}

var cronLogsCmd = &cobra.Command{
	Use:   "cron-logs",
	Short: "View background job execution logs",
	Run: func(cmd *cobra.Command, args []string) {
		pterm.Info.Println("Fetching latest daemon logs from systemd journal...")

		journalCmd := exec.Command("journalctl", "-u", "devops.service", "-n", "50", "--no-pager")
		output, err := journalCmd.CombinedOutput()

		if err != nil {
			pterm.Error.Printf("Failed to fetch logs: %v\n", err)
			pterm.Warning.Println("Are you running on a system with systemd and is devops.service installed?")
			return
		}

		if len(output) == 0 {
			pterm.Warning.Println("No logs found for devops.service in the system journal.")
			return
		}

		fmt.Println("\n" + string(output))
	},
}

var cronRemoveCmd = &cobra.Command{
	Use:   "cron-remove",
	Short: "Remove a scheduled background job (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Remove Cron Job")

		idRaw, cancelled := askInt("Cron Job ID to remove")
		if cancelled {
			return
		}
		if idRaw == 0 {
			pterm.Error.Println("Valid Cron Job ID is required.")
			return
		}

		var job models.Cronjob
		if err := database.DB.First(&job, idRaw).Error; err != nil {
			pterm.Error.Printf("Cron job with ID %d not found.\n", idRaw)
			return
		}

		fmt.Println()
		pterm.Warning.Printf("You are about to remove job: '%s' [%s]\n", job.Name, job.Schedule)
		confirmed, cancelled := askConfirm("Are you sure?")
		if cancelled || !confirmed {
			pterm.Info.Println("Cancelled.")
			return
		}

		if err := database.DB.Delete(&job).Error; err != nil {
			pterm.Error.Println("Failed to delete cron job:", err)
			return
		}

		pterm.Success.Printf("Cron job '%s' (ID: %d) has been removed.\n", job.Name, idRaw)
	},
}

func init() {
	rootCmd.AddCommand(cronListCmd)
	rootCmd.AddCommand(cronAddCmd)
	rootCmd.AddCommand(cronToggleCmd)
	rootCmd.AddCommand(cronRemoveCmd)
	rootCmd.AddCommand(cronLogsCmd)
}
