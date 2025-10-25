package cmd

import (
	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "hibana",
	Short: "Hibana Stack - Configure your Ubuntu server with DNS, mail, and multi-domain support",
	Long: `Hibana Stack is a CLI tool that automates the complete setup of an Ubuntu server with:
- PowerDNS for authoritative DNS management
- Mail server with DMARC and DKIM configuration
- Traefik reverse proxy for multi-domain hosting with automatic SSL
- Web admin interface for easy domain and mail management`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./hibana-config.json)")
}
