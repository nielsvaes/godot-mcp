package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/tomyud1/godot-mcp/cli/internal/config"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running gdcli daemon",
		RunE: func(c *cobra.Command, _ []string) error {
			url := fmt.Sprintf("http://127.0.0.1:%d/shutdown", config.HTTPPort())
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Post(url, "application/json", nil)
			if err != nil {
				fmt.Fprintln(c.OutOrStdout(), "No gdcli daemon is running.")
				return nil
			}
			resp.Body.Close()
			fmt.Fprintln(c.OutOrStdout(), "Daemon shutdown requested.")
			return nil
		},
	}
}
