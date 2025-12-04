package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SetupTraefik configures Traefik reverse proxy
func (i *Installer) SetupTraefik() error {
	// Open firewall ports for web services
	if err := i.openWebPorts(); err != nil {
		fmt.Printf("Warning: failed to open web ports in firewall: %v\n", err)
		fmt.Println("   You may need to open ports manually: 80 (HTTP), 443 (HTTPS)")
	}

	traefikDir := "/etc/hibana/traefik"

	// Create Traefik directories
	if err := os.MkdirAll(filepath.Join(traefikDir, "config"), 0755); err != nil {
		return fmt.Errorf("failed to create Traefik directory: %w", err)
	}

	// Create acme.json for Let's Encrypt
	acmeFile := filepath.Join(traefikDir, "acme.json")
	if _, err := os.Stat(acmeFile); os.IsNotExist(err) {
		if err := os.WriteFile(acmeFile, []byte(""), 0600); err != nil {
			return fmt.Errorf("failed to create acme.json: %w", err)
		}
	}

	// Create Traefik static configuration
	if err := i.createTraefikConfig(traefikDir); err != nil {
		return fmt.Errorf("failed to create Traefik config: %w", err)
	}

	// Create dynamic configuration
	if err := i.createTraefikDynamicConfig(traefikDir); err != nil {
		return fmt.Errorf("failed to create Traefik dynamic config: %w", err)
	}

	return nil
}

// createTraefikConfig creates the static Traefik configuration
func (i *Installer) createTraefikConfig(dir string) error {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	email := fmt.Sprintf("admin@%s", primaryDomain.Name)

	config := fmt.Sprintf(`# Traefik Static Configuration
[global]
  checkNewVersion = true
  sendAnonymousUsage = false

[log]
  level = "INFO"

[api]
  dashboard = true
  insecure = false

[entryPoints]
  [entryPoints.web]
    address = ":80"
    [entryPoints.web.http]
      [entryPoints.web.http.redirections]
        [entryPoints.web.http.redirections.entryPoint]
          to = "websecure"
          scheme = "https"

  [entryPoints.websecure]
    address = ":443"
    [entryPoints.websecure.http]
      [entryPoints.websecure.http.tls]
        certResolver = "letsencrypt"

[certificatesResolvers.letsencrypt]
  [certificatesResolvers.letsencrypt.acme]
    email = "%s"
    storage = "/etc/traefik/acme.json"
    [certificatesResolvers.letsencrypt.acme.httpChallenge]
      entryPoint = "web"

[providers]
  [providers.docker]
    endpoint = "unix:///var/run/docker.sock"
    exposedByDefault = false
    network = "hibana-network"

  [providers.file]
    directory = "/etc/traefik/config"
    watch = true
`, email)

	configPath := filepath.Join(dir, "traefik.yml")
	return os.WriteFile(configPath, []byte(config), 0644)
}

// createTraefikDynamicConfig creates dynamic configuration for Traefik
func (i *Installer) createTraefikDynamicConfig(dir string) error {
	config := `# Traefik Dynamic Configuration
http:
  routers: {}
  services: {}
  middlewares:
    security-headers:
      headers:
        frameDeny: true
        sslRedirect: true
        browserXssFilter: true
        contentTypeNosniff: true
        forceSTSHeader: true
        stsIncludeSubdomains: true
        stsPreload: true
        stsSeconds: 31536000
`

	configPath := filepath.Join(dir, "config", "dynamic.yml")
	return os.WriteFile(configPath, []byte(config), 0644)
}

// DeployContainers deploys Docker containers for web services
func (i *Installer) DeployContainers() error {
	composeDir := "/etc/hibana/docker"

	if err := os.MkdirAll(composeDir, 0755); err != nil {
		return fmt.Errorf("failed to create docker directory: %w", err)
	}

	// Create Docker network
	if err := i.createDockerNetwork(); err != nil {
		return fmt.Errorf("failed to create Docker network: %w", err)
	}

	// Create docker-compose file
	if err := i.createDockerCompose(composeDir); err != nil {
		return fmt.Errorf("failed to create docker-compose file: %w", err)
	}

	// Create Dockerfiles and content
	if err := i.createWebContainers(composeDir); err != nil {
		return fmt.Errorf("failed to create web containers: %w", err)
	}

	// Start containers
	if err := i.startDockerCompose(composeDir); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	return nil
}

// createDockerNetwork creates the Docker network for Hibana
func (i *Installer) createDockerNetwork() error {
	// Check if network exists
	cmd := exec.Command("docker", "network", "inspect", "hibana-network")
	if err := cmd.Run(); err == nil {
		// Network already exists
		return nil
	}

	// Create network
	cmd = exec.Command("docker", "network", "create", "hibana-network")
	return cmd.Run()
}

// createDockerCompose creates the docker-compose.yml file
func (i *Installer) createDockerCompose(dir string) error {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	domain := primaryDomain.Name

	compose := fmt.Sprintf(`version: '3.8'

services:
  traefik:
    image: traefik:v2.10
    container_name: hibana-traefik
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /etc/hibana/traefik/traefik.yml:/etc/traefik/traefik.yml:ro
      - /etc/hibana/traefik/config:/etc/traefik/config:ro
      - /etc/hibana/traefik/acme.json:/etc/traefik/acme.json
    networks:
      - hibana-network
    labels:
      - "traefik.enable=true"

  www:
    build: ./www
    container_name: hibana-www
    restart: unless-stopped
    networks:
      - hibana-network
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.www.rule=Host(` + "`www.%s`, `%s`)" + `"
      - "traefik.http.routers.www.entrypoints=websecure"
      - "traefik.http.routers.www.tls.certresolver=letsencrypt"
      - "traefik.http.services.www.loadbalancer.server.port=80"

  adm:
    build: ./adm
    container_name: hibana-adm
    restart: unless-stopped
    networks:
      - hibana-network
    environment:
      - DOMAIN=%s
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.adm.rule=Host(` + "`adm.%s`)" + `"
      - "traefik.http.routers.adm.entrypoints=websecure"
      - "traefik.http.routers.adm.tls.certresolver=letsencrypt"
      - "traefik.http.services.adm.loadbalancer.server.port=80"

  webmail:
    image: roundcube/roundcubemail:latest
    container_name: hibana-webmail
    restart: unless-stopped
    networks:
      - hibana-network
    environment:
      - ROUNDCUBEMAIL_DEFAULT_HOST=mail.%s
      - ROUNDCUBEMAIL_SMTP_SERVER=mail.%s
      - ROUNDCUBEMAIL_DEFAULT_PORT=143
      - ROUNDCUBEMAIL_SMTP_PORT=587
      - ROUNDCUBEMAIL_UPLOAD_MAX_FILESIZE=30M
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.webmail.rule=Host(` + "`webmail.%s`)" + `"
      - "traefik.http.routers.webmail.entrypoints=websecure"
      - "traefik.http.routers.webmail.tls.certresolver=letsencrypt"
      - "traefik.http.services.webmail.loadbalancer.server.port=80"

networks:
  hibana-network:
    external: true
`, domain, domain, domain, domain, domain, domain, domain)

	composePath := filepath.Join(dir, "docker-compose.yml")
	return os.WriteFile(composePath, []byte(compose), 0644)
}

// createWebContainers creates Dockerfiles and content for www and adm
func (i *Installer) createWebContainers(dir string) error {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	// Create www container
	wwwDir := filepath.Join(dir, "www")
	if err := os.MkdirAll(wwwDir, 0755); err != nil {
		return err
	}

	wwwDockerfile := `FROM nginx:alpine
COPY index.html /usr/share/nginx/html/index.html
EXPOSE 80
`

	wwwIndex := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to %s</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
        }
        .container {
            text-align: center;
        }
        h1 {
            font-size: 3em;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Hello World!</h1>
        <p>Welcome to %s</p>
        <p>Powered by Hibana Stack</p>
    </div>
</body>
</html>
`, primaryDomain.Name, primaryDomain.Name)

	if err := os.WriteFile(filepath.Join(wwwDir, "Dockerfile"), []byte(wwwDockerfile), 0644); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(wwwDir, "index.html"), []byte(wwwIndex), 0644); err != nil {
		return err
	}

	// Create adm container
	admDir := filepath.Join(dir, "adm")
	if err := os.MkdirAll(admDir, 0755); err != nil {
		return err
	}

	admDockerfile := `FROM nginx:alpine
COPY index.html /usr/share/nginx/html/index.html
EXPOSE 80
`

	admIndex := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Hibana Stack - Admin</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #1e3c72 0%%, #2a5298 100%%);
            color: white;
        }
        .container {
            text-align: center;
        }
        h1 {
            font-size: 3em;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Hibana Stack Admin</h1>
        <p>Administration interface for %s</p>
        <p>Coming soon in Phase 2</p>
    </div>
</body>
</html>
`, primaryDomain.Name)

	if err := os.WriteFile(filepath.Join(admDir, "Dockerfile"), []byte(admDockerfile), 0644); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(admDir, "index.html"), []byte(admIndex), 0644); err != nil {
		return err
	}

	return nil
}

// startDockerCompose starts the Docker containers
func (i *Installer) startDockerCompose(dir string) error {
	cmd := exec.Command("docker-compose", "up", "-d", "--build")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// openWebPorts opens firewall ports for web services (HTTP/HTTPS)
func (i *Installer) openWebPorts() error {
	fmt.Println("  Opening firewall ports for web services...")

	// Open web ports
	// 80/tcp - HTTP (for Let's Encrypt challenges and HTTP to HTTPS redirect)
	// 443/tcp - HTTPS (for secure web access)
	ports := []string{"80/tcp", "443/tcp"}
	portDescriptions := map[string]string{
		"80/tcp":  "HTTP - Let's Encrypt challenges and redirects",
		"443/tcp": "HTTPS - secure web access",
	}

	return i.openPortsWithTracking("web_setup", ports, portDescriptions)
}
