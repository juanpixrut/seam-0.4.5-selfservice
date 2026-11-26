package accounting

import "testing"

// planDeCuentas is the table of section 12.3 of the design's "Plan de cuentas
// y contraparte de los movimientos": code, name, whether it takes movements.
// 2.1.n (each real AE) is created at authorization and is not in the initial
// chart.
var planDeCuentas = []struct {
	code, name string
	postable   bool
}{
	{"1", "Activo", false}, {"1.1", "Disponibilidades", false},
	{"1.1.1", "Banco", true}, {"1.1.2", "PayPal", true}, {"1.1.3", "dLocal Go", true},
	{"2", "Pasivo", false}, {"2.1", "Comisiones a pagar a AE", false},
	{"2.1.1", "Seam AE", true}, {"2.1.2", "Seam Web", true},
	{"2.2", "Acreedores y proveedores", false},
	{"3", "Patrimonio", false}, {"3.1", "Aportes y retiros de socios", false},
	{"3.1.1", "Shareholder1", true}, {"3.1.2", "Shareholder2", true},
	{"4", "Ingresos", false}, {"4.1", "Ingresos por servicios", true}, {"4.2", "Otros ingresos", true},
	{"5", "Egresos", false}, {"5.1", "Comisiones de AE", true}, {"5.2", "Gastos de cobranza", true},
	{"5.3", "Gastos de operación", true}, {"5.4", "Impuestos y tasas", true}, {"5.5", "Otros egresos y ajustes", true},
}

func TestDefaultChartMatchesThePlanDeCuentas(t *testing.T) {
	got := map[string]Account{}
	for _, a := range DefaultChart() {
		got[a.Code] = a
	}
	if len(got) != len(planDeCuentas) {
		t.Errorf("the chart has %d accounts, the plan has %d", len(got), len(planDeCuentas))
	}
	for _, w := range planDeCuentas {
		a, ok := got[w.code]
		switch {
		case !ok:
			t.Errorf("account %s %s is missing", w.code, w.name)
		case a.Name != w.name:
			t.Errorf("account %s is %q, the plan says %q", w.code, a.Name, w.name)
		case a.Postable != w.postable:
			t.Errorf("account %s postable = %v, the plan says %v", w.code, a.Postable, w.postable)
		}
	}
	if _, err := NewChart(DefaultChart()); err != nil {
		t.Fatal(err)
	}
}

// Section 12.4: which side increases each group.
func TestGroupNaturesMatchThePlan(t *testing.T) {
	for _, c := range []struct {
		code  string
		debit bool
	}{{"1", true}, {"2", false}, {"3", false}, {"4", false}, {"5", true}} {
		if got := GroupOfCode(c.code).DebitNormal(); got != c.debit {
			t.Errorf("group %s debit-normal = %v, want %v", c.code, got, c.debit)
		}
	}
}
