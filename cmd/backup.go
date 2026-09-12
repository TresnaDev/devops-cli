package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func getBackupDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devops", "backups")
}

var backupListCmd = &cobra.Command{
	Use:   "backup-list",
	Short: "List all downloaded server backups in local OS",
	Run: func(cmd *cobra.Command, args []string) {
		bDir := getBackupDir()
		
		// If a server ID is provided, filter the directory to that server only
		if backupServerID != 0 {
			var server models.Server
			if err := database.DB.First(&server, backupServerID).Error; err == nil {
				bDir = filepath.Join(bDir, server.Name)
			} else {
				pterm.Error.Printf("Server with ID %d not found.\n", backupServerID)
				return
			}
		}

		os.MkdirAll(bDir, 0755)

		tableData := pterm.TableData{
			{"SERVER (FOLDER)", "FILE NAME", "SIZE", "LAST MODIFIED"},
		}

		fileCount := 0
		baseDir := getBackupDir() // Always use root backup dir for relative paths
		filepath.Walk(bDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			
			relPath, _ := filepath.Rel(baseDir, path)
			dirName := filepath.Dir(relPath)
			if dirName == "." {
				dirName = "Root"
			}
			
			sizeMB := float64(info.Size()) / 1024.0 / 1024.0
			tableData = append(tableData, []string{
				dirName,
				info.Name(),
				fmt.Sprintf("%.2f MB", sizeMB),
				info.ModTime().Format("02 Jan 2006 15:04"),
			})
			fileCount++
			return nil
		})

		if fileCount == 0 {
			pterm.Info.Println("No backups found in", bDir)
			return
		}

		fmt.Println()
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithHeaderRowSeparator("═").WithRowSeparator("─").
			WithHeaderStyle(pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)).
			WithData(tableData).Render()
		pterm.FgGray.Printf("\nBackup Directory: %s\n\n", bDir)
	},
}

var (
	backupServerID   int
	backupRemotePath string
	backupLocalPath  string
	backupRemoteUser string
)

var backupCreateCmd = &cobra.Command{
	Use:   "backup-create",
	Short: "Pull backup files from a remote server using Rsync",
	Run: func(cmd *cobra.Command, args []string) {
		serverIDRaw := backupServerID
		remotePath := backupRemotePath
		localDest := backupLocalPath
		sshUser := backupRemoteUser
		
		// Only run interactive prompts if running without flags
		if serverIDRaw == 0 && remotePath == "" {
			printFormHeader("Pull Backup")

			var cancelled bool
			serverIDRaw, cancelled = askInt("Target Server ID")
			if cancelled {
				return
			}
			
			if serverIDRaw == 0 {
				pterm.Error.Println("Valid Server ID is required.")
				return
			}

			// Fetch server early to use its name for the default path
			var server models.Server
			if err := database.DB.First(&server, serverIDRaw).Error; err != nil {
				pterm.Error.Printf("Server with ID %d not found.\n", serverIDRaw)
				return
			}
			
			sshUser, cancelled = askDefault("Remote SSH user", server.User)
			if cancelled {
				return
			}

			pterm.FgGray.Println("  Example remote path: /var/backups/app_db.sql")
			remotePath, cancelled = ask("Remote path to pull")
			if cancelled {
				return
			}
			
			defaultLocal := filepath.Join(getBackupDir(), server.Name)
			localDest, cancelled = askDefault("Local destination", defaultLocal)
			if cancelled {
				return
			}
			fmt.Println()
		}

		if serverIDRaw == 0 {
			pterm.Error.Println("Valid Server ID is required.")
			return
		}
		if remotePath == "" {
			pterm.Error.Println("Remote path is required.")
			return
		}

		var server models.Server
		if err := database.DB.First(&server, serverIDRaw).Error; err != nil {
			pterm.Error.Printf("Server with ID %d not found.\n", serverIDRaw)
			return
		}

		if sshUser == "" {
			sshUser = server.User
		}

		if localDest == "" {
			localDest = filepath.Join(getBackupDir(), server.Name)
		}

		os.MkdirAll(localDest, 0755)

		// Setup SSH Key path
		home, _ := os.UserHomeDir()
		privKeyPath := filepath.Join(home, ".devops", "ssh", "id_ed25519")

		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Pulling backups from %s (%s)...", server.Name, server.Host))

		sshOpts := fmt.Sprintf("ssh -i %s -p %d -o StrictHostKeyChecking=accept-new -o BatchMode=yes -o IdentitiesOnly=yes", privKeyPath, server.Port)
		remoteTarget := fmt.Sprintf("%s@%s:%s", sshUser, server.Host, remotePath)

		rsyncCmd := exec.Command("rsync", "-avz", "-e", sshOpts, remoteTarget, localDest)
		
		output, err := rsyncCmd.CombinedOutput()
		if err != nil {
			spinner.Fail("Backup failed!")
			pterm.Error.Println(string(output))
			return
		}

		spinner.Success(fmt.Sprintf("Backup pulled successfully from %s to %s!", server.Name, localDest))
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "backup-restore",
	Short: "Restore a server from backup / Push local files to remote",
	Run: func(cmd *cobra.Command, args []string) {
		serverIDRaw := backupServerID
		localPath := backupLocalPath
		remotePath := backupRemotePath
		sshUser := backupRemoteUser
		
		// Only run interactive prompts if running without flags
		if serverIDRaw == 0 && localPath == "" && remotePath == "" {
			printFormHeader("Restore / Push Backup")

			var cancelled bool
			serverIDRaw, cancelled = askInt("Target Server ID")
			if cancelled {
				return
			}
			
			if serverIDRaw == 0 {
				pterm.Error.Println("Valid Server ID is required.")
				return
			}

			// Fetch server early to use its name for the default path
			var server models.Server
			if err := database.DB.First(&server, serverIDRaw).Error; err != nil {
				pterm.Error.Printf("Server with ID %d not found.\n", serverIDRaw)
				return
			}
			
			sshUser, cancelled = askDefault("Remote SSH user", server.User)
			if cancelled {
				return
			}
			
			localPath, cancelled = ask("Local source path")
			if cancelled {
				return
			}
			
			remotePath, cancelled = ask("Remote destination path")
			if cancelled {
				return
			}
			fmt.Println()
		}

		if serverIDRaw == 0 {
			pterm.Error.Println("Valid Server ID is required.")
			return
		}
		if localPath == "" {
			pterm.Error.Println("Local source path is required.")
			return
		}
		if remotePath == "" {
			pterm.Error.Println("Remote destination path is required.")
			return
		}

		var server models.Server
		if err := database.DB.First(&server, serverIDRaw).Error; err != nil {
			pterm.Error.Printf("Server with ID %d not found.\n", serverIDRaw)
			return
		}

		if sshUser == "" {
			sshUser = server.User
		}

		// Setup SSH Key path
		home, _ := os.UserHomeDir()
		privKeyPath := filepath.Join(home, ".devops", "ssh", "id_ed25519")

		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Pushing backups to %s (%s)...", server.Name, server.Host))

		sshOpts := fmt.Sprintf("ssh -i %s -p %d -o StrictHostKeyChecking=accept-new -o BatchMode=yes -o IdentitiesOnly=yes", privKeyPath, server.Port)
		remoteTarget := fmt.Sprintf("%s@%s:%s", sshUser, server.Host, remotePath)

		// rsync local to remote
		rsyncCmd := exec.Command("rsync", "-avz", "-e", sshOpts, localPath, remoteTarget)
		
		output, err := rsyncCmd.CombinedOutput()
		if err != nil {
			spinner.Fail("Restore failed!")
			pterm.Error.Println(string(output))
			return
		}

		spinner.Success(fmt.Sprintf("Backup pushed successfully to %s!", server.Name))
	},
}

func init() {
	backupListCmd.Flags().IntVarP(&backupServerID, "server-id", "s", 0, "Filter backups by Server ID (optional)")

	backupCreateCmd.Flags().IntVarP(&backupServerID, "server-id", "s", 0, "Target Server ID (required)")
	backupCreateCmd.Flags().StringVarP(&backupRemotePath, "remote", "r", "", "Remote path to pull (required, e.g. /var/backups/)")
	backupCreateCmd.Flags().StringVarP(&backupLocalPath, "local", "l", "", "Local destination (default: ~/.devops/backups/)")
	backupCreateCmd.Flags().StringVarP(&backupRemoteUser, "user", "u", "", "Remote SSH user (defaults to server base user)")

	backupRestoreCmd.Flags().IntVarP(&backupServerID, "server-id", "s", 0, "Target Server ID (required)")
	backupRestoreCmd.Flags().StringVarP(&backupLocalPath, "local", "l", "", "Local source path (required)")
	backupRestoreCmd.Flags().StringVarP(&backupRemotePath, "remote", "r", "", "Remote destination path (required)")
	backupRestoreCmd.Flags().StringVarP(&backupRemoteUser, "user", "u", "", "Remote SSH user (defaults to server base user)")

	rootCmd.AddCommand(backupListCmd)
	rootCmd.AddCommand(backupCreateCmd)
	rootCmd.AddCommand(backupRestoreCmd)
}
