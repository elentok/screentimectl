package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

const (
	serviceUser   = "screentimectl"
	configDir     = "/etc/screentimectl"
	sudoersPath   = "/etc/sudoers.d/screentimectl"
	servicePath   = "/etc/systemd/system/screentimectl.service"
	pamService    = "/etc/pam.d/gdm-password"
	pamRule       = "auth required pam_exec.so /usr/local/bin/screentimectl check-login"
	exampleConfig = `machine_name: "My-PC"

telegram:
  bot_token: "YOUR_BOT_TOKEN_HERE"
  allowed_chat_ids:
    - 111111111

server:
  listen_addr: "127.0.0.1:3847"

notifications:
  thresholds: [30, 15, 5, 1]

users:
  - name: "user1"
    daily_limit_minutes: 300
    allowed_hours:
      start: 8
      end: 18
`
	serviceFile = `[Unit]
Description=screentimectl daemon
After=network.target

[Service]
Type=simple
User=screentimectl
ExecStart=/usr/local/bin/screentimectl run
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`
	sudoersContent = `screentimectl ALL=(ALL) NOPASSWD: /usr/bin/loginctl
screentimectl ALL=(ALL) NOPASSWD: /usr/bin/chage -E 0 *, /usr/bin/chage -E -1 *
screentimectl ALL=(ALL) NOPASSWD: SETENV: /usr/bin/notify-send, /usr/bin/espeak-ng
`
)

func runSetup() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("setup must be run as root")
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"create system user", createSystemUser},
		{"create config directory", createConfigDir},
		{"create data directory", createDataDir},
		{"install sudoers rule", installSudoers},
		{"install PAM rule", installPAMRule},
		{"install systemd service", installService},
		{"reload systemd", reloadSystemd},
	}

	for _, step := range steps {
		fmt.Printf("  %s... ", step.name)
		if err := step.fn(); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Println("OK")
	}

	fmt.Printf("\nSetup complete.\n")
	fmt.Printf("Edit %s/config.yaml, then run:\n", configDir)
	fmt.Printf("  systemctl enable --now screentimectl\n")
	return nil
}

func createSystemUser() error {
	// Check if user already exists
	if err := exec.Command("id", serviceUser).Run(); err == nil {
		return nil // already exists
	}
	return exec.Command("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", serviceUser).Run()
}

func createConfigDir() error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	cfgFile := configDir + "/config.yaml"
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		if err := os.WriteFile(cfgFile, []byte(exampleConfig), 0640); err != nil {
			return err
		}
	}
	return chownToServiceUser(configDir, cfgFile)
}

func createDataDir() error {
	if err := os.MkdirAll(usageDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	return chownToServiceUser(usageDir, logDir)
}

func installPAMRule() error {
	data, err := os.ReadFile(pamService)
	if err != nil {
		return fmt.Errorf("reading %s: %w", pamService, err)
	}
	if strings.Contains(string(data), "screentimectl") {
		return nil // already installed
	}
	// Prepend the rule before the first auth line
	lines := strings.Split(string(data), "\n")
	var result []string
	inserted := false
	for _, line := range lines {
		if !inserted && strings.HasPrefix(strings.TrimSpace(line), "auth") {
			result = append(result, pamRule)
			inserted = true
		}
		result = append(result, line)
	}
	if !inserted {
		result = append([]string{pamRule}, result...)
	}
	return os.WriteFile(pamService, []byte(strings.Join(result, "\n")), 0644)
}

func installSudoers() error {
	return os.WriteFile(sudoersPath, []byte(sudoersContent), 0440)
}

func installService() error {
	// Write the binary path based on our own executable
	self, err := os.Executable()
	if err != nil {
		self = "/usr/local/bin/screentimectl"
	}

	content := fmt.Sprintf(`[Unit]
Description=screentimectl daemon
After=network.target

[Service]
Type=simple
User=screentimectl
ExecStart=%s run
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, self)

	return os.WriteFile(servicePath, []byte(content), 0644)
}

func chownToServiceUser(paths ...string) error {
	u, err := user.Lookup(serviceUser)
	if err != nil {
		return fmt.Errorf("looking up user %s: %w", serviceUser, err)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	for _, p := range paths {
		if err := os.Chown(p, uid, gid); err != nil {
			return err
		}
	}
	return nil
}

func reloadSystemd() error {
	// systemctl daemon-reload fails when systemd is not PID 1 (e.g. in containers).
	link, err := os.Readlink("/proc/1/exe")
	if err != nil || !strings.Contains(link, "systemd") {
		return nil
	}
	return exec.Command("systemctl", "daemon-reload").Run()
}
