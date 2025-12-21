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

	// Check if pre-built files exist in project root (from setup.sh)
	prebuiltDir := filepath.Join(projectRoot, "ansible", "api-build")
	prebuiltAPI := filepath.Join(prebuiltDir, "hibana-api")
	prebuiltWeb := filepath.Join(prebuiltDir, "web")
	prebuiltDockerfile := filepath.Join(prebuiltDir, "Dockerfile")

	if _, err := os.Stat(prebuiltAPI); err == nil {
		if _, err := os.Stat(prebuiltWeb); err == nil {
			// Pre-built files exist, copy them instead of rebuilding
			fmt.Println("  → Using pre-built API and frontend from setup.sh...")

			// Copy API binary
			if err := copyFile(prebuiltAPI, filepath.Join(buildDir, "hibana-api")); err != nil {
				return fmt.Errorf("failed to copy pre-built API: %w", err)
			}

			// Copy web directory
			if err := copyDir(prebuiltWeb, filepath.Join(buildDir, "web")); err != nil {
				return fmt.Errorf("failed to copy pre-built frontend: %w", err)
			}

			// Copy Dockerfile if it exists
			if _, err := os.Stat(prebuiltDockerfile); err == nil {
				if err := copyFile(prebuiltDockerfile, filepath.Join(buildDir, "Dockerfile")); err != nil {
					return fmt.Errorf("failed to copy pre-built Dockerfile: %w", err)
				}
			}

			fmt.Println("    ✓ Pre-built API and frontend copied")
			return nil
		}
	}

	// Step 1: Build Go API binary (static for Alpine)
	fmt.Println("  → Building Go API...")
	apiBinary := filepath.Join(buildDir, "hibana-api")
	buildCmd := exec.Command("go", "build", "-buildvcs=false", "-ldflags=-s -w", "-o", apiBinary, "./cmd/api")
	buildCmd.Dir = projectRoot
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build API: %w", err)
	}
	fmt.Println("    ✓ API binary built (static)")

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

RUN apk --no-cache add ca-certificates docker-cli docker-cli-compose

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

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Get source file info for permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Write to destination with same permissions
	return os.WriteFile(dst, data, srcInfo.Mode())
}
