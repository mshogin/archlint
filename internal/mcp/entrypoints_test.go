package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

func fn(id, title string) model.Node    { return model.Node{ID: id, Title: title, Entity: "function"} }
func meth(id, title string) model.Node  { return model.Node{ID: id, Title: title, Entity: "method"} }
func typ(id, title string) model.Node   { return model.Node{ID: id, Title: title, Entity: "struct"} }

// (1) main -> в R; (2) exported func -> в R; (3) unexported не-init/не-Test -> НЕ в дефолтном R.
func TestEntryPoints_Default(t *testing.T) {
	g := &model.Graph{Nodes: []model.Node{
		fn("cmd/app.main", "main"),
		fn("p.PublicAPI", "PublicAPI"),
		fn("p.helper", "helper"),
		fn("p.init", "init"),
		fn("p.TestFoo", "TestFoo"),
		meth("p.S.Exported", "Exported"),
		meth("p.S.internal", "internal"),
		typ("p.PublicType", "PublicType"),
		typ("p.privateType", "privateType"),
	}}
	r := EntryPoints(g, nil)

	for _, want := range []string{"cmd/app.main", "p.PublicAPI", "p.init", "p.TestFoo", "p.S.Exported", "p.PublicType"} {
		if !r[want] {
			t.Errorf("%s должен быть в дефолтном R", want)
		}
	}
	for _, notWant := range []string{"p.helper", "p.S.internal", "p.privateType"} {
		if r[notWant] {
			t.Errorf("%s НЕ должен быть в дефолтном R (unexported, не-entry)", notWant)
		}
	}
}

// (4) конфиг-маркер (подстрока) ДОБАВЛЯЕТ узел в R сверх дефолта.
func TestEntryPoints_ConfigAdds(t *testing.T) {
	g := &model.Graph{Nodes: []model.Node{
		fn("p.handleWebhook", "handleWebhook"), // unexported -> не в дефолте
	}}
	if EntryPoints(g, nil)["p.handleWebhook"] {
		t.Fatal("без конфига handleWebhook не в R")
	}
	r := EntryPoints(g, []string{"handleWebhook"})
	if !r["p.handleWebhook"] {
		t.Fatal("конфиг-паттерн handleWebhook должен добавить узел в R")
	}
}
