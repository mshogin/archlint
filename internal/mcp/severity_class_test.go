package mcp
import ("testing";"github.com/mshogin/archlint/internal/archlintcfg")
func TestSeverityClass_DeadCode(t *testing.T){
  c,ok:=ClassOf("dead-code")
  if !ok || c.Class!="ERROR" || !c.OpenWorld || !c.RequiresDelta || !c.HumanInLoop {
    t.Fatalf("dead-code класс должен быть ERROR/open-world/delta/human-in-loop; %+v ok=%v",c,ok)
  }
  // эффективный уровень = аудит (Telemetry, не блок) до Фазы 5
  cfg:=archlintcfg.Default()
  if lvl:=ViolationLevel(Violation{Kind:"dead-code"},&cfg); lvl!=archlintcfg.LevelTelemetry {
    t.Fatalf("dead-code эффективный уровень = аудит Telemetry до дельта-инфры; got %v",lvl)
  }
}

// Единый severity-реестр SSOT: downgrade-вердикты (INFO/WARNING) в одной точке.
func TestSeverityClass_Registry(t *testing.T) {
	cfg := archlintcfg.Default()

	// INFO (магнитуды/дубли) -> LevelPersonal ([INFO]), убраны из WARNING-шума.
	infoKinds := []string{
		"srp-multiple-responsibilities", "srp-too-many-methods", "srp-too-many-fields",
		"god-class", "hub-node", "high-efferent-coupling",
	}
	for _, k := range infoKinds {
		if !IsInfoClass(k) {
			t.Errorf("%s должен быть INFO в едином реестре", k)
		}
		if lvl := ViolationLevel(Violation{Kind: k}, &cfg); lvl != archlintcfg.LevelPersonal {
			t.Errorf("%s -> LevelPersonal ([INFO]), got %v", k, lvl)
		}
	}

	// WARNING (доказуемые сигналы).
	for _, k := range []string{"dip-concrete-dependency", "srp-lack-of-cohesion", "structural-clone"} {
		if !IsWarningClass(k) {
			t.Errorf("%s должен быть WARNING в едином реестре", k)
		}
	}

	// ERROR не задет downgrade'ом.
	for _, k := range []string{"circular-dependency", "dead-code", "isp-usage-subset"} {
		if SeverityClassOf(k) != "ERROR" {
			t.Errorf("%s должен остаться ERROR, got %q", k, SeverityClassOf(k))
		}
	}
}
