package cmd

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the background task scheduler (for Systemd/24-7 usage)",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("========================================")
		log.Println("🚀 Starting DevOps CLI Daemon Engine...")
		log.Println("========================================")

		// Create a new internal Go cron scheduler
		c := cron.New()

		// Fetch all active jobs from our SQLite database
		var jobs []models.Cronjob
		if err := database.DB.Where("is_active = ?", true).Find(&jobs).Error; err != nil {
			log.Fatalf("Failed to fetch jobs: %v\n", err)
		}

		if len(jobs) == 0 {
			log.Println("No active jobs found in database. Daemon will sleep.")
		}

		// Inject jobs into the in-memory scheduler
		for _, j := range jobs {
			jobID := j.ID
			commandStr := j.Command
			jobName := j.Name

			_, err := c.AddFunc(j.Schedule, func() {
				log.Printf("[Job %d | %s] Executing command: %s\n", jobID, jobName, commandStr)

				// Execute the command string via the host's shell
				cmdRun := exec.Command("/bin/sh", "-c", commandStr)
				output, err := cmdRun.CombinedOutput()

				if err != nil {
					log.Printf("[Job %d | %s] ❌ FAILED: %v\nOutput: %s\n", jobID, jobName, err, string(output))
				} else {
					log.Printf("[Job %d | %s] ✅ SUCCESS\n", jobID, jobName)
				}

				// Update LastRun timestamp in database
				var updateJob models.Cronjob
				if database.DB.First(&updateJob, jobID).Error == nil {
					updateJob.LastRun = time.Now()
					database.DB.Save(&updateJob)
				}
			})

			if err != nil {
				log.Printf("❌ Failed to schedule job ID %d ('%s'): %v\n", jobID, jobName, err)
			} else {
				log.Printf("✅ Scheduled Job: '%s' [%s] -> %s\n", jobName, j.Schedule, commandStr)
			}
		}

		// Start the scheduler asynchronously
		c.Start()

		// Wait indefinitely until termination signal (e.g. from Systemd or Ctrl+C)
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig

		log.Println("\n🛑 Shutting down DevOps Daemon Engine gracefully...")
		c.Stop()
		os.Exit(0)
	},
}

func init() {
	// Register it to the root command (note: we do NOT add it to the interactive REPL TAB completion)
	rootCmd.AddCommand(daemonCmd)
}
