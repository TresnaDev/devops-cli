package cmd

import (
	"fmt"
	"os"
	"time"

	"devops-cli/internal/crypto"
	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	gossh "golang.org/x/crypto/ssh"
)


// ── cred-list ──────────────────────────────────────────────────────────────────

var credListCmd = &cobra.Command{
	Use:   "cred-list",
	Short: "List all SSH credentials for a server",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Credential List")

		serverID, cancelled := askInt("Server ID")
		if cancelled {
			return
		}
		if serverID == 0 {
			pterm.Error.Println("Valid Server ID is required.")
			return
		}

		var s models.Server
		if err := database.DB.First(&s, serverID).Error; err != nil {
			pterm.Error.Printf("Server with ID %d not found.\n", serverID)
			return
		}

		var creds []models.ServerCredential
		database.DB.Where("server_id = ?", serverID).Find(&creds)

		if len(creds) == 0 {
			pterm.Info.Printf("No credentials configured for server '%s'. Add one with 'cred-add'.\n", s.Name)
			return
		}

		pterm.Info.Printf("Credentials for server: %s (%s)\n\n", s.Name, s.Host)

		tableData := pterm.TableData{
			{"ID", "LABEL", "USER", "AUTH METHOD", "STATUS", "LAST CHECKED"},
		}
		for _, c := range creds {
			authMethod := "password"
			if c.SSHPassword == "" && c.SSHKeyPath != "" {
				authMethod = "key"
			} else if c.SSHPassword != "" && c.SSHKeyPath != "" {
				authMethod = "password+key"
			}

			statusStr := pterm.FgGray.Sprint("unchecked")
			if c.Status == "valid" {
				statusStr = pterm.FgGreen.Sprint("VALID ✓")
			} else if c.Status == "invalid" {
				statusStr = pterm.FgRed.Sprint("INVALID ✗")
			}

			lastChecked := "-"
			if c.LastChecked != nil {
				lastChecked = c.LastChecked.Format("02 Jan 06 15:04")
			}

			tableData = append(tableData, []string{
				fmt.Sprintf("%d", c.ID),
				c.Label,
				c.User,
				authMethod,
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

var credAddCmd = &cobra.Command{
	Use:   "cred-add",
	Short: "Add an SSH credential to a server (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Add Server Credential")

		serverID, cancelled := askInt("Server ID")
		if cancelled {
			return
		}
		if serverID == 0 {
			pterm.Error.Println("Valid Server ID is required.")
			return
		}

		var s models.Server
		if err := database.DB.First(&s, serverID).Error; err != nil {
			pterm.Error.Printf("Server with ID %d not found.\n", serverID)
			return
		}

		pterm.Info.Printf("Adding credential to server: %s (%s)\n\n", s.Name, s.Host)

		user, cancelled := ask("SSH Username")
		if cancelled {
			return
		}
		if user == "" {
			pterm.Error.Println("SSH Username is required.")
			return
		}

		password, cancelled := askSecretOptional("SSH Password")
		if cancelled {
			return
		}
		keyPath, cancelled := askOptional("SSH Key Path (e.g. ~/.devops/ssh/id_ed25519)")
		if cancelled {
			return
		}

		if password == "" && keyPath == "" {
			pterm.Error.Println("At least one auth method required: SSH Password or Key Path.")
			return
		}

		authType := "password"
		if password == "" && keyPath != "" {
			authType = "key"
		} else if password != "" && keyPath != "" {
			authType = "password+key"
		}
		defaultLabel := fmt.Sprintf("%s (%s)", user, authType)
		label, cancelled := askDefault("Label", defaultLabel)
		if cancelled {
			return
		}

		fmt.Println()
		newCred := models.ServerCredential{
			ServerID:   uint(serverID),
			Label:      label,
			User:       user,
			SSHKeyPath: keyPath,
		}

		if password != "" {
			encrypted, err := crypto.Encrypt(password)
			if err != nil {
				pterm.Error.Println("Failed to encrypt password:", err)
				return
			}
			newCred.SSHPassword = encrypted
		}

		if err := database.DB.Create(&newCred).Error; err != nil {
			pterm.Error.Println("Failed to save credential:", err)
			return
		}

		pterm.Success.Printf("Credential '%s' added to server '%s' (ID: %d)\n", label, s.Name, newCred.ID)
		pterm.Info.Println("Run 'check-credential' to validate it.")
	},
}

var credRemoveCmd = &cobra.Command{
	Use:   "cred-remove",
	Short: "Remove a credential from a server (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Remove Credential")

		idRaw, cancelled := askInt("Credential ID to remove")
		if cancelled {
			return
		}
		if idRaw == 0 {
			pterm.Error.Println("Valid Credential ID is required.")
			return
		}

		var cred models.ServerCredential
		if err := database.DB.First(&cred, idRaw).Error; err != nil {
			pterm.Error.Printf("Credential with ID %d not found.\n", idRaw)
			return
		}

		fmt.Println()
		pterm.Warning.Printf("You are about to remove credential: '%s' (User: %s)\n", cred.Label, cred.User)
		confirmed, cancelled := askConfirm("Are you sure?")
		if cancelled || !confirmed {
			pterm.Info.Println("Cancelled.")
			return
		}

		if err := database.DB.Delete(&cred).Error; err != nil {
			pterm.Error.Println("Failed to delete credential:", err)
			return
		}

		pterm.Success.Printf("Credential '%s' (ID: %d) has been removed.\n", cred.Label, idRaw)
	},
}


// ── trySSHCredential ───────────────────────────────────────────────────────────

func trySSHCredential(c *models.ServerCredential, host string, port int) error {
	var authMethods []gossh.AuthMethod

	if c.SSHPassword != "" {
		password, err := crypto.Decrypt(c.SSHPassword)
		if err != nil {
			return fmt.Errorf("failed to decrypt stored password")
		}
		authMethods = append(authMethods, gossh.Password(password))
	}

	if c.SSHKeyPath != "" {
		keyPath := c.SSHKeyPath
		if len(keyPath) > 1 && keyPath[:2] == "~/" {
			home, _ := os.UserHomeDir()
			keyPath = home + keyPath[1:]
		}
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("cannot read key file: %s", keyPath)
		}
		signer, err := gossh.ParsePrivateKey(keyBytes)
		if err != nil {
			return fmt.Errorf("invalid private key file: %v", err)
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	}

	config := &gossh.ClientConfig{
		User:            c.User,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         8 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	client.Close()
	return nil
}

func init() {
	rootCmd.AddCommand(credListCmd)
	rootCmd.AddCommand(credAddCmd)
	rootCmd.AddCommand(credRemoveCmd)
}
