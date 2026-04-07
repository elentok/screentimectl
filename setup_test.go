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

func TestTrayDependenciesIncludePythonGI(t *testing.T) {
	for _, pkg := range []string{"python3-gi", "gir1.2-gtk-3.0", "gir1.2-ayatanaappindicator3-0.1"} {
		if !strings.Contains(trayDependencyPackages, pkg) {
			t.Fatalf("trayDependencyPackages = %q, want %s", trayDependencyPackages, pkg)
		}
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
}
