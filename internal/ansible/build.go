package ansible

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// BuildAPIAndFrontend builds the API binary and React frontend
func BuildAPIAndFrontend(workspaceDir string) error {
	// Get the project root directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create build directory in workspace
	buildDir := filepath.Join(workspaceDir, "api-build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Step 1: Build Go API binary
	fmt.Println("  → Building Go API...")
	apiBinary := filepath.Join(buildDir, "hibana-api")
	buildCmd := exec.Command("go", "build", "-o", apiBinary, "./cmd/api")
	buildCmd.Dir = projectRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build API: %w", err)
	}
	fmt.Println("    ✓ API binary built")

	// Step 2: Build React frontend
	fmt.Println("  → Building React admin interface...")
	frontendDir := filepath.Join(projectRoot, "web", "admin")

	// Check if node_modules exists
	nodeModules := filepath.Join(frontendDir, "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		fmt.Println("    → Installing npm dependencies...")
		npmInstall := exec.Command("npm", "install")
		npmInstall.Dir = frontendDir
		npmInstall.Stdout = os.Stdout
		npmInstall.Stderr = os.Stderr

		if err := npmInstall.Run(); err != nil {
			return fmt.Errorf("failed to install npm dependencies: %w", err)
		}
	}

	// Run npm build
	npmBuild := exec.Command("npm", "run", "build")
	npmBuild.Dir = frontendDir
	npmBuild.Stdout = os.Stdout
	npmBuild.Stderr = os.Stderr

	if err := npmBuild.Run(); err != nil {
		return fmt.Errorf("failed to build React app: %w", err)
	}

	// Check if dist directory was created
	distSrc := filepath.Join(frontendDir, "dist")
	if _, err := os.Stat(distSrc); os.IsNotExist(err) {
		return fmt.Errorf("frontend dist directory not created - build may have failed")
	}

	// Copy dist to build directory
	distDest := filepath.Join(buildDir, "web")
	if err := os.MkdirAll(distDest, 0755); err != nil {
		return fmt.Errorf("failed to create web directory: %w", err)
	}

	if err := copyDir(distSrc, distDest); err != nil {
		return fmt.Errorf("failed to copy frontend dist: %w", err)
	}

	fmt.Println("    ✓ Frontend built")

	// Step 3: Create Dockerfile for binary deployment
	dockerfileContent := `FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy pre-built binary
COPY hibana-api .

# Copy web admin interface
COPY web ./web

# Expose API port
EXPOSE 3000

CMD ["./hibana-api", "--port", "3000", "--static", "./web"]
`
	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}

	fmt.Println("    ✓ Dockerfile created")
	fmt.Printf("    Build artifacts ready in: %s\n", buildDir)

	return nil
}
