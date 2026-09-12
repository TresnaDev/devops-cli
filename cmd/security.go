package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var securityKeygenCmd = &cobra.Command{
	Use:   "security-keygen",
	Short: "Generate a dedicated SSH Keypair for this agent",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}

		// Create an isolated SSH directory specifically for the DevOps CLI
		sshDir := filepath.Join(home, ".devops", "ssh")
		if err := os.MkdirAll(sshDir, 0700); err != nil {
			pterm.Error.Println("Failed to create SSH directory:", err)
			return
		}

		privKeyPath := filepath.Join(sshDir, "id_ed25519")
		pubKeyPath := privKeyPath + ".pub"

		// Check if key already exists
		if _, err := os.Stat(privKeyPath); err == nil {
			pterm.Warning.Printf("SSH Key already exists at: %s\n", privKeyPath)
			pterm.Info.Println("Using the existing key. If you wish to regenerate, delete the file manually first.")
		} else {
			spinner, _ := pterm.DefaultSpinner.Start("Generating ultra-secure Ed25519 SSH Keypair...")
			
			// Execute OS ssh-keygen with empty passphrase (-N "") and custom comment (-C)
			keygen := exec.Command("ssh-keygen", "-t", "ed25519", "-f", privKeyPath, "-N", "", "-C", "devops-cli-agent", "-q")
			if err := keygen.Run(); err != nil {
				spinner.Fail("Failed to generate SSH key: ", err)
				return
			}
			spinner.Success("SSH Keypair generated successfully!")
		}

		// Read and display the public key for the user
		pubKey, err := os.ReadFile(pubKeyPath)
		if err != nil {
			pterm.Error.Println("Failed to read public key:", err)
			return
		}

		pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgLightYellow)).Println("Your Agent's Public Key (Gembok)")
		pterm.Info.Println("Tugas Anda: Copy teks berwarna hijau di bawah ini, lalu paste ke dalam file")
		pterm.Info.Println("~/.ssh/authorized_keys milik user 'bizops-agent' di SEMUA server Anda.")
		
		fmt.Println()
		pterm.FgGreen.Println(string(pubKey))
		
		pterm.FgGray.Printf("Private Key (Kunci Master) disimpan super aman di: %s\n\n", privKeyPath)
	},
}

func init() {
	rootCmd.AddCommand(securityKeygenCmd)
}
