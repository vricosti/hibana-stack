package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vricosti/hibana-stack/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long:  `Manage Hibana Stack configuration files`,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a skeleton configuration file",
	Long:  `Creates a skeleton configuration file with example values that you can customize`,
	RunE:  runGenerate,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("output", "o", "hibana-config.json", "Output file path")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	outputFile, _ := cmd.Flags().GetString("output")

	// Check if file already exists
	if _, err := os.Stat(outputFile); err == nil {
		return fmt.Errorf("file already exists: %s (delete it first or use a different output path)", outputFile)
	}

	// Generate skeleton
	skeleton := config.GenerateSkeleton()
	data, err := json.MarshalIndent(skeleton, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Configuration skeleton created: %s\n\n", outputFile)
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit the configuration file with your domain and server details")
	fmt.Println("  2. Update email accounts with secure passwords")
	fmt.Println("  3. Run 'sudo hibana init' to install and configure your server")

	return nil
}
