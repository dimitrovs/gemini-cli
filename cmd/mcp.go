package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP servers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("mcp command")
	},
}

var mcpAddCmd = &cobra.Command{
	Use:   "add <name> <commandOrUrl> [args...]",
	Short: "Add a server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("mcp add command")
	},
}

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("mcp remove command")
	},
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured MCP servers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("mcp list command")
	},
}
