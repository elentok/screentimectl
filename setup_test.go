package main

import (
	"strings"
	"testing"
)

func TestPAMRuleShowsFriendlyOutput(t *testing.T) {
	if !strings.Contains(pamRule, " quiet ") {
		t.Fatalf("pamRule = %q, want quiet option", pamRule)
	}
	if !strings.Contains(pamRule, " stdout ") {
		t.Fatalf("pamRule = %q, want stdout option", pamRule)
	}
}

func TestTrayScriptUsesCompactStatus(t *testing.T) {
	if !strings.Contains(trayScript, `"/usr/local/bin/screentimectl", "status", "--compact"`) {
		t.Fatalf("trayScript does not call compact status")
	}
	if !strings.Contains(trayScript, "AyatanaAppIndicator3") {
		t.Fatalf("trayScript does not use AyatanaAppIndicator3")
	}
	if !strings.HasPrefix(trayScript, "#!/usr/bin/python3\n") {
		t.Fatalf("trayScript does not use system python")
	}
}

func TestAptDependenciesIncludeRequiredRuntimePackages(t *testing.T) {
	for _, pkg := range []string{"sudo", "libnotify-bin", "pulseaudio-utils", "python3-venv", "gnome-shell-extension-appindicator", "python3-gi", "gir1.2-gtk-3.0", "gir1.2-ayatanaappindicator3-0.1"} {
		if !strings.Contains(aptDependencyPackages, pkg) {
			t.Fatalf("aptDependencyPackages = %q, want %s", aptDependencyPackages, pkg)
		}
	}
	if strings.Contains(aptDependencyPackages, "espeak-ng") {
		t.Fatalf("aptDependencyPackages should not contain espeak-ng")
	}
}

func TestSetupAssets(t *testing.T) {
	for name, content := range map[string]string{
		"exampleConfig":  exampleConfig,
		"serviceFile":    serviceFile,
		"sudoersContent": sudoersContent,
		"trayScript":     trayScript,
	} {
		if strings.TrimSpace(content) == "" {
			t.Fatalf("%s is empty", name)
		}
	}
	if !strings.Contains(exampleConfig, `machine_name: "My-PC"`) {
		t.Fatalf("exampleConfig missing machine name")
	}
	if !strings.Contains(serviceFile, "ExecStart=/usr/local/bin/screentimectl run") {
		t.Fatalf("serviceFile missing ExecStart")
	}
	if !strings.Contains(sudoersContent, "/usr/bin/chage -E 0 *") {
		t.Fatalf("sudoersContent missing chage rule")
	}
	if !strings.Contains(sudoersContent, "/usr/bin/paplay") {
		t.Fatalf("sudoersContent missing paplay rule")
	}
	if strings.Contains(sudoersContent, "espeak-ng") {
		t.Fatalf("sudoersContent should not contain espeak-ng")
	}
}
