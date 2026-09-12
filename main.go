package main

import (
	"devops-cli/cmd"
	"devops-cli/internal/database"
	"os"
)

func main() {
	// Initialize Local SQLite DB before running commands
	database.InitDB()

	if len(os.Args) == 1 {
		cmd.RunREPL()
	} else {
		cmd.Execute()
	}
}
