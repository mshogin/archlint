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
