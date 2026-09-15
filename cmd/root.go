package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/google/shlex"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

var rootCmd = &cobra.Command{
	Use:   "devops",
	Short: "DevOps Assistant CLI",
	Long:  `DevOps Assistant CLI - Terminal Interface for Infrastructure Automation`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func printAsciiHeaderOnly() {
	headerAscii := `
 ██████╗ ███████╗██╗   ██╗██████╗ ██████╗ ███████╗      ██████╗ ██╗     ██╗
 ██╔══██╗██╔════╝██║   ██║██╔═══██╗██╔══██╗██╔════╝     ██╔════╝ ██║     ██║
 ██║  ██║█████╗  ██║   ██║██║   ██║██████╔╝███████╗     ██║      ██║     ██║
 ██║  ██║██╔══╝  ╚██╗ ██╔╝██║   ██║██╔═══╝ ╚════██║     ██║      ██║     ██║
 ██████╔╝███████╗ ╚████╔╝ ╚██████╔╝██║     ███████║     ╚██████╗ ███████╗██║
 ╚═════╝ ╚══════╝  ╚═══╝   ╚═════╝ ╚═╝     ╚══════╝      ╚═════╝ ╚══════╝╚═╝
`
	pterm.FgLightYellow.Println(strings.TrimPrefix(headerAscii, "\n"))

	info := pterm.LightYellow("  Author  ") + pterm.FgWhite.Sprint("TresnaAgustina") +
		pterm.FgGray.Sprint("   │   ") +
		pterm.LightYellow("Version  ") + pterm.FgWhite.Sprint("1.0.0") +
		pterm.FgGray.Sprint("   │   ") +
		pterm.LightYellow("DB  ") + pterm.FgWhite.Sprint("~/.devops/devops.db") +
		pterm.FgGray.Sprint("   │   ") +
		pterm.LightYellow("Daemon  ") + pterm.FgWhite.Sprint("systemd (devops.service)") + "\n"

	tips := pterm.FgGray.Sprint("  TAB: autocomplete   ·   exit / quit: keluar   ·   clear: bersihkan layar")

	fmt.Println(info)
	fmt.Println(tips)
	fmt.Println()
}

// Store the original terminal state
var initialTermState *term.State

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}

func executor(in string) {
	in = strings.TrimSpace(in)
	if in == "" {
		return
	}
	
	if in == "exit" || in == "quit" {
		pterm.FgGreen.Println("Goodbye! 👋")
		// Restore terminal manually before exiting
		if initialTermState != nil {
			term.Restore(int(os.Stdin.Fd()), initialTermState)
		}
		os.Exit(0)
	}

	if in == "clear" {
		fmt.Print("\033[H\033[2J")
		printAsciiHeaderOnly()
		return
	}

	args, err := shlex.Split(in)
	if err != nil {
		pterm.Error.Println("Invalid command syntax (unclosed quotes?):", err)
		return
	}
	
	if len(args) > 0 && args[0] == "devops" {
		args = args[1:]
	}

	// Reset all flags to their default values before executing
	// to prevent previous REPL commands from leaking state.
	resetFlags(rootCmd)

	rootCmd.SetArgs(args)
	rootCmd.Execute()
}

func completer(d prompt.Document) []prompt.Suggest {
	if d.TextBeforeCursor() == "" {
		return []prompt.Suggest{}
	}

	s := []prompt.Suggest{
		// Server Management
		{Text: "server-list", Description: "List all configured servers"},
		{Text: "server-add", Description: "Add server  [--name --host --user --port --password --key-path]"},
		{Text: "server-import", Description: "Bulk add servers from a CSV file"},
		{Text: "server-edit", Description: "Edit server [--id --name --host --user --port --password --key-path]"},
		{Text: "server-remove", Description: "Remove server [--id]"},
		{Text: "server-history", Description: "View event log (DOWN/UP/INVALID) for a single server [--id]"},

		// Credential Management (per server)
		{Text: "cred-list", Description: "List all SSH credentials for a server [--server-id]"},
		{Text: "cred-add", Description: "Add SSH credential to a server [--server-id --user --password/--key-path]"},
		{Text: "cred-remove", Description: "Remove a credential [--id]"},

		// Health Checks
		{Text: "check-tcp", Description: "Run TCP health check on all active servers [--id]"},
		{Text: "check-credential", Description: "Validate ALL SSH credentials for all servers [--id]"},
		{Text: "check-history", Description: "View history of health & credential checks"},
		{Text: "monitor-latency", Description: "Monitor server latency in real-time"},

		// Cron / Scheduler
		{Text: "cron-list", Description: "List all scheduled background jobs"},
		{Text: "cron-add", Description: "Add cron job [--name --schedule --cmd]"},
		{Text: "cron-remove", Description: "Remove cron job [--id]"},
		{Text: "cron-toggle", Description: "Toggle cron job active/paused [--id]"},
		{Text: "cron-logs", Description: "View background job execution logs"},

		// Backup
		{Text: "backup-list", Description: "List server backups [--server-id]"},
		{Text: "backup-create", Description: "Pull backup files from a remote server via rsync"},
		{Text: "backup-restore", Description: "Restore a server from backup"},

		// HTTP Endpoint Monitoring
		{Text: "endpoint-list", Description: "List all monitored HTTP/HTTPS endpoints"},
		{Text: "endpoint-add", Description: "Add a new HTTP/HTTPS endpoint to monitor"},
		{Text: "endpoint-remove", Description: "Remove a monitored endpoint"},
		{Text: "endpoint-toggle", Description: "Toggle an endpoint active/paused"},
		{Text: "endpoint-history", Description: "View event history for a specific endpoint [--id]"},
		{Text: "check-http", Description: "Run HTTP health check on all active endpoints [--id]"},

		// Security
		{Text: "security-keygen", Description: "Generate Ed25519 SSH Keypair for the agent"},

		// Config
		{Text: "config-alert", Description: "Configure Telegram bot for alerts [--tg-token --tg-chatid]"},
		{Text: "config-view", Description: "View current Telegram alert configuration"},
		{Text: "test-alert", Description: "Send a test message to verify Telegram alert works"},

		// Shell
		{Text: "clear", Description: "Clear the terminal screen"},
		{Text: "help", Description: "Show all available commands"},
		{Text: "exit", Description: "Exit the DevOps CLI"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func RunREPL() {
	// Save the terminal state BEFORE go-prompt starts
	if state, err := term.GetState(int(os.Stdin.Fd())); err == nil {
		initialTermState = state
	}

	printHermesStyleBanner()

	p := prompt.New(
		executor,
		completer,
		prompt.OptionPrefix("❯ "),
		prompt.OptionPrefixTextColor(prompt.Yellow),
		prompt.OptionSuggestionBGColor(prompt.Black),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionDescriptionBGColor(prompt.Black),
		prompt.OptionDescriptionTextColor(prompt.DarkGray),
		prompt.OptionSelectedSuggestionBGColor(prompt.Yellow),
		prompt.OptionSelectedSuggestionTextColor(prompt.Black),
		prompt.OptionSelectedDescriptionBGColor(prompt.Yellow),
		prompt.OptionSelectedDescriptionTextColor(prompt.Black),
		prompt.OptionShowCompletionAtStart(), 
	)
	p.Run()
}

func applyGradient(ascii string) string {
	lines := strings.Split(strings.TrimRight(ascii, "\n"), "\n")
	var result strings.Builder

	startR, startG, startB := float64(255), float64(235), float64(50)
	endR, endG, endB := float64(140), float64(110), float64(0)

	total := len(lines) - 1
	if total <= 0 {
		total = 1
	}

	for i, line := range lines {
		ratio := float64(i) / float64(total)
		r := uint8(startR + ratio*(endR-startR))
		g := uint8(startG + ratio*(endG-startG))
		b := uint8(startB + ratio*(endB-startB))

		coloredLine := pterm.NewRGB(r, g, b).Sprint(line)
		result.WriteString(coloredLine + "\n")
	}
	return result.String()
}

func printHermesStyleBanner() {
	printAsciiHeaderOnly()

	rawAscii := `⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡀⠀⠀⠀⠀⠀⢀⠀⠀⠀
⠀⠀⠀⢂⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣶⠛⠁⠀⠀⡆⠀⢀⣼⠀⠀⠀
⠀⠀⠀⢾⠀⠀⠀⠀⠀⢀⢠⠂⠀⠀⠀⠀⣿⣇⡄⠀⠀⣰⠇⠀⠘⠀⠀⠀⠀
⠀⠀⠀⢠⠀⠀⠀⠀⠀⢸⣄⠀⠀⢘⡄⢰⣿⣿⣣⣴⠁⢁⢀⢀⠀⠀⠀⢀⠆
⢠⠀⠀⠘⠧⡀⠀⠰⡄⠈⢻⡄⢸⣿⣿⣿⣿⣿⣳⠋⣠⣷⠘⣸⠀⢠⡇⠈⠀
⠰⣇⠀⢧⢠⠘⣦⠀⠀⡘⣾⣞⣿⣿⣿⣿⡿⢟⣿⣿⢿⣟⢀⠟⡄⠈⡇⠀⠀
⠀⠨⡆⠀⠗⠛⠸⡇⢳⣷⣻⡟⠟⡿⣵⣯⣾⢿⣿⣧⣼⣿⢺⢠⣷⠀⠁⠀⠀
⠰⢸⡇⢀⠈⠳⠶⠽⣆⣻⡟⣿⣇⣿⣿⣿⡏⣵⣿⡿⠛⠛⣾⢸⣣⠀⠀⠀⠀
⠀⠀⢉⣾⡄⡁⣲⠫⢡⠘⣷⣿⡿⣧⢇⢿⣷⣿⠟⣡⠀⠀⣷⣟⡇⠀⠀⠀⠀
⠀⠀⢰⣟⠛⠲⢤⣤⡤⠂⢄⢎⣿⣿⣶⣍⢍⡀⠀⠀⣀⣼⣿⡿⠇⠀⠀⠀⠀
⠀⠀⢦⠙⢌⢭⡷⠾⠶⠶⢂⡲⢨⣿⣷⣝⠾⣝⡲⠶⢚⣫⣿⡃⢮⢢⠀⠀⠀
⠀⠀⠈⠳⠀⠀⠀⠀⣤⣼⠷⣗⠘⣿⣿⠛⡳⠉⠻⢿⣿⢵⣮⡛⢤⣃⠇⠀⠀
⠀⠀⠀⠀⠈⠀⠀⢈⣬⡝⣻⣛⠄⢛⣃⣐⡴⣭⣍⠳⡅⠀⠉⠲⠶⠟⠀⠀⠀
⠀⠀⠀⠀⣀⣤⣶⡿⠛⢁⠀⠀⠀⠀⠀⠀⠀⡌⠛⠿⣿⣦⣄⡀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠘⢿⣤⣘⣷⠶⠚⠛⠻⢶⣭⣁⣴⠖⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠈⠙⠞⣼⣿⣾⣿⣞⠖⠉⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠈⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀`

	leftAscii := applyGradient(rawAscii)

	leftInfo := "\n" +
		pterm.LightYellow("Author: ") + "TresnaAgustina\n" +
		pterm.LightYellow("Version: ") + "1.0.0 (2026)\n" +
		pterm.LightYellow("System: ") + "Linux/Amd64\n" +
		pterm.FgGray.Sprint("Session: db_local_sqlite")

	leftPanel := leftAscii + leftInfo

	rightContent := "\n\n\n" +
		pterm.LightYellow("▸ Server Management") + "\n" +
		pterm.FgWhite.Sprint("  server-list") + pterm.FgGray.Sprint("                     List all servers\n") +
		pterm.FgWhite.Sprint("  server-add ") + pterm.FgGray.Sprint(" --name --host --user --port\n") +
		pterm.FgWhite.Sprint("             ") + pterm.FgGray.Sprint(" --password --key-path\n") +
		pterm.FgWhite.Sprint("  server-import") + pterm.FgGray.Sprint(" <file.csv>\n") +
		pterm.FgWhite.Sprint("  server-edit") + pterm.FgGray.Sprint(" --id [--name --host --user --port]\n") +
		pterm.FgWhite.Sprint("             ") + pterm.FgGray.Sprint(" [--password --key-path]\n") +
		pterm.FgWhite.Sprint("  server-remove") + pterm.FgGray.Sprint(" --id\n") +
		pterm.FgWhite.Sprint("  server-history") + pterm.FgGray.Sprint(" --id\n") +
		"\n" +
		pterm.LightYellow("▸ Health Checks") + "\n" +
		pterm.FgWhite.Sprint("  check-tcp") + pterm.FgGray.Sprint("          TCP ping all active servers [--id]\n") +
		pterm.FgWhite.Sprint("  check-credential") + pterm.FgGray.Sprint("   Validate SSH credentials [--id]\n") +
		pterm.FgWhite.Sprint("  check-history") + pterm.FgGray.Sprint("      View last check results\n") +
		"\n" +
		pterm.LightYellow("▸ Scheduler") + "\n" +
		pterm.FgWhite.Sprint("  cron-list") + pterm.FgGray.Sprint("          List all cron jobs\n") +
		pterm.FgWhite.Sprint("  cron-add ") + pterm.FgGray.Sprint(" --name --schedule --cmd\n") +
		pterm.FgWhite.Sprint("  cron-remove") + pterm.FgGray.Sprint(" --id\n") +
		pterm.FgWhite.Sprint("  cron-toggle") + pterm.FgGray.Sprint(" --id\n") +
		"\n" +
		pterm.LightYellow("▸ Security & Config") + "\n" +
		pterm.FgWhite.Sprint("  security-keygen") + pterm.FgGray.Sprint("    Generate Ed25519 SSH keypair\n") +
		pterm.FgWhite.Sprint("  config-alert") + pterm.FgGray.Sprint(" --token --chat-id\n") +
		"\n" +
		pterm.FgGray.Sprint("  TAB: autocomplete  |  exit: quit")

	panels := pterm.Panels{
		{
			{Data: leftPanel},
			{Data: rightContent},
		},
	}
	
	renderedPanels, _ := pterm.DefaultPanel.WithPanels(panels).WithPadding(6).Srender()
	
	box := pterm.DefaultBox.
		WithTitle(" DevOps CLI v2.0.0 ").
		WithTitleTopLeft().
		WithBoxStyle(pterm.NewStyle(pterm.FgLightYellow)).
		Sprint(renderedPanels)
		
	fmt.Println(box)
}
