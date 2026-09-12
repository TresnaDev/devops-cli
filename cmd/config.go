package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"devops-cli/internal/database"
	"devops-cli/internal/models"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// upsertSetting saves or updates a key-value setting in the database.
func upsertSetting(key, value string) {
	var setting models.Setting
	database.DB.Where(models.Setting{Key: key}).FirstOrCreate(&setting)
	setting.Value = value
	database.DB.Save(&setting)
}

var configAlertCmd = &cobra.Command{
	Use:   "config-alert",
	Short: "Configure Telegram bot credentials for alerts (interactive)",
	Run: func(cmd *cobra.Command, args []string) {
		printFormHeader("Configure Telegram Alert")

		pterm.FgGray.Println("  Get your Bot Token from @BotFather on Telegram.")
		token, cancelled := ask("Telegram Bot Token")
		if cancelled {
			return
		}

		pterm.FgGray.Println("  Get your Chat ID by sending a message to your bot and checking:")
		pterm.FgGray.Println("  https://api.telegram.org/bot<TOKEN>/getUpdates")
		chatID, cancelled := ask("Telegram Chat ID")
		if cancelled {
			return
		}

		if token == "" && chatID == "" {
			pterm.Error.Println("At least one value (Token or Chat ID) is required.")
			return
		}

		fmt.Println()
		if token != "" {
			upsertSetting("telegram_token", token)
			pterm.Success.Println("Telegram Bot Token saved.")
		}
		if chatID != "" {
			upsertSetting("telegram_chat_id", chatID)
			pterm.Success.Println("Telegram Chat ID saved.")
		}

		pterm.Info.Println("Configuration saved! Run 'test-alert' to verify.")
	},
}

var configViewCmd = &cobra.Command{
	Use:   "config-view",
	Short: "View current alert configuration",
	Run: func(cmd *cobra.Command, args []string) {
		token := database.GetSetting("telegram_token")
		chatID := database.GetSetting("telegram_chat_id")

		fmt.Println()
		if token == "" {
			pterm.Warning.Println("Telegram Token  : (not set)")
		} else {
			// Mask token for security: show first 10 chars only
			masked := token[:10] + "..." + token[len(token)-4:]
			pterm.Info.Println("Telegram Token  :", masked)
		}

		if chatID == "" {
			pterm.Warning.Println("Telegram Chat ID: (not set)")
		} else {
			pterm.Info.Println("Telegram Chat ID:", chatID)
		}
		fmt.Println()
	},
}

var testAlertCmd = &cobra.Command{
	Use:   "test-alert",
	Short: "Send a test Telegram alert to verify configuration",
	Run: func(cmd *cobra.Command, args []string) {
		token := database.GetSetting("telegram_token")
		chatID := database.GetSetting("telegram_chat_id")

		if token == "" || chatID == "" {
			pterm.Error.Println("Telegram not configured. Run 'config-alert' to set it up.")
			return
		}

		spinner, _ := pterm.DefaultSpinner.Start("Sending test alert to Telegram...")

		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
		message := fmt.Sprintf(
			"✅ <b>TEST ALERT — DevOps CLI</b>\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"🎉 Konfigurasi Telegram berhasil!\n"+
				"Alert akan terkirim ke chat ini saat:\n"+
				"  • Server DOWN terdeteksi\n"+
				"  • Credential SSH invalid\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"🕐 <code>%s</code>\n\n"+
				"<i>— DevOps CLI Monitor</i>",
			time.Now().Format("02 Jan 2006, 15:04:05 WIB"),
		)

		payload := map[string]string{
			"chat_id":    chatID,
			"text":       message,
			"parse_mode": "HTML",
		}

		jsonPayload, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			spinner.Fail(fmt.Sprintf("HTTP request failed: %v", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			spinner.Success("Test alert sent successfully! Check your Telegram.")
		} else {
			spinner.Fail(fmt.Sprintf("Telegram API returned status %d — check your token and chat ID.", resp.StatusCode))
		}
	},
}

func init() {
	rootCmd.AddCommand(configAlertCmd)
	rootCmd.AddCommand(configViewCmd)
	rootCmd.AddCommand(testAlertCmd)
}
