// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0
// https://www.apache.org/licenses/LICENSE-2.0

// Command accounting-seedgen writes seeds/accounting/seed-accounting.seed.
//
// The books of the Accounting & Admin module cannot be typed by hand: an item
// that is collected books an entry and accrues a commission, a settlement
// claims lines and is paid with another entry, a close keeps the balances of a
// month. So the data is made the way the application makes it: this command
// runs the module's own Service over a store in memory, with a clock it moves
// through five months of 2026, and then writes every record the Service left
// as a .seed file (docs/SEED_FORMAT.md). No xolu is contacted.
//
// Ids are not written: every record is found by a field of its own ("match")
// and every reference to another record is a binding, so the file loads over
// an instance where the module has already started and made its chart.
//
//	go run ./cmd/accounting-seedgen > seeds/accounting/seed-accounting.seed
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	acc "github.com/ha1tch/seam-ui/internal/accounting"
)

// sandbox is the tenant whose AE is the developer account.
const sandbox = "sandbox"

// The people of the seed. Their ids here stand for the ids xolu gives them at
// load time: every one written by the Service is replaced by a binding.
const (
	uStaff int64 = 900001 + iota
	uAccountant
	uMarta
	uDiego
	uPaula
	uDev
)

type person struct {
	id          int64
	as          string
	name, email string
	role        string
}

var people = []person{
	{uStaff, "u_staff", "Admin Staff", "accounting.staff@example.com", "admin_staff"},
	{uAccountant, "u_accountant", "Accountant", "accountant@example.com", "accountant"},
	{uMarta, "u_marta", "Marta Rivas", "marta.rivas@example.com", "account_executive"},
	{uDiego, "u_diego", "Diego Suárez", "diego.suarez@example.com", "account_executive"},
	{uPaula, "u_paula", "Paula Méndez", "paula.mendez@example.com", "account_executive"},
	// The developer account of the other seeds (found by its email when one of
	// them is loaded, created here otherwise). With the Developer module on it
	// can view the application as each role above; so that viewing as an AE
	// shows a portal with something in it, it is itself an authorized AE with a
	// customer of its own (the sandbox tenant, below).
	{uDev, "u_dev", "Dev", "dev@ual.li", "developer"},
}

// bcrypt of the demonstration password every seed uses.
const passwordHash = "$2b$10$C.6TAiQOamXbGpzcHhmoR.mGswuyJJyk8OwSdWlC6O4IT8oqmtUWS"

// ---------------------------------------------------------------------------
// a platform that answers from a table, and a place for documents
// ---------------------------------------------------------------------------

type platform struct {
	month      string // items up to this period are reported
	tenants    []acc.TenantSnapshot
	items      []acc.ItemSnapshot
	inProgress map[string]bool
}

func (p *platform) Pull(context.Context) (acc.InstanceSnapshot, error) {
	snap := acc.InstanceSnapshot{Tenants: p.tenants}
	for _, it := range p.items {
		if it.Period > p.month {
			continue
		}
		if p.inProgress[it.ExternalRef] {
			it.State = acc.ItemInProgress
		}
		snap.Items = append(snap.Items, it)
	}
	return snap, nil
}

func (p *platform) MarkCollected(context.Context, string) error { return nil }

type memBlobs struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (b *memBlobs) Put(_ context.Context, key string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.m[key] = data
	return nil
}

func (b *memBlobs) Get(_ context.Context, key string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.m[key], nil
}

func (b *memBlobs) Delete(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.m, key)
	return nil
}

// A file the Service accepts as a PDF. The seed carries the record of each
// document and not its content, which Seam keeps on disk.
var pdf = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Count 0/Kids[]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")

func must[T any](v T, err error) T {
	if err != nil {
		fmt.Fprintln(os.Stderr, "accounting-seedgen:", err)
		os.Exit(1)
	}
	return v
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "accounting-seedgen:", err)
		os.Exit(1)
	}
}

func usd(s string) acc.Cents { return must(acc.ParseUSD(s)) }

func main() {
	ctx := context.Background()
	store := acc.NewMemStore()
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	at := func(day string, hour int) {
		d := must(time.Parse(acc.DateLayout, day))
		now = d.Add(time.Duration(hour) * time.Hour)
	}
	pf := &platform{inProgress: map[string]bool{}}
	svc := acc.NewService(store, acc.WithInstance(pf), acc.WithBlobs(&memBlobs{m: map[string][]byte{}}), acc.WithClock(func() time.Time { return now }))

	// --- July 1: the module starts; the company and the formula rules ---
	check(svc.Bootstrap(ctx))
	check(svc.SaveCompany(ctx, acc.Company{LegalName: "Seam Demo S.A.", TaxID: "RUT 210000010019", Address: "Av. Ejemplo 1234, Montevideo", Country: "UY", BillingEmail: "billing@example.com"}))
	for _, f := range []acc.Formula{
		{Situation: acc.SituationCommission, Name: "Real AE: 10% of what is collected", Enabled: true, Effect: acc.EffectSet, Method: acc.MethodPercent, BPS: 1000, AEKind: "real"},
		{Situation: acc.SituationCommission, Name: "Real AE: bonus for a startup", Enabled: true, Effect: acc.EffectSurcharge, Method: acc.MethodFixed, Amount: usd("25.00"), AEKind: "real", Concept: acc.ConceptStartup},
		{Situation: acc.SituationCommission, Name: "Seam AE: 5% of what is collected", Enabled: true, Effect: acc.EffectSet, Method: acc.MethodPercent, BPS: 500, AEKind: "seam_ae"},
		{Situation: acc.SituationPrice, Name: "Paraguay: 20% off", Enabled: true, Effect: acc.EffectDiscount, Method: acc.MethodPercent, BPS: 2000, Country: "PY"},
		{Situation: acc.SituationPrice, Name: "Japan: 5% surcharge", Enabled: false, Effect: acc.EffectSurcharge, Method: acc.MethodPercent, BPS: 500, Country: "JP"},
	} {
		must(svc.SaveFormula(ctx, f, 0))
	}

	// --- July 2 and 3: two AEs register, say how they are paid, and are authorized ---
	at("2026-07-02", 10)
	marta := must(svc.RegisterAE(ctx, uMarta, "Marta Rivas", "marta.rivas@example.com"))
	diego := must(svc.RegisterAE(ctx, uDiego, "Diego Suárez", "diego.suarez@example.com"))
	must(svc.SaveAEProfile(ctx, marta.ID, acc.AEProfile{Name: marta.Name, Email: marta.Email, Countries: []string{"UY", "PY"}, PayoutMethod: acc.PayoutTransfer, PayoutDetails: "Banco Ejemplo, caja de ahorro USD 000-0000000 (demo)"}))
	must(svc.SaveAEProfile(ctx, diego.ID, acc.AEProfile{Name: diego.Name, Email: diego.Email, Countries: []string{"UY", "ES"}, PayoutMethod: acc.PayoutPayPal, PayoutDetails: "diego.suarez@example.com"}))
	at("2026-07-03", 11)
	must(svc.AuthorizeAE(ctx, marta.ID, uStaff))
	must(svc.AuthorizeAE(ctx, diego.ID, uStaff))
	// the developer's own AE, for viewing the portal as one (see people)
	at("2026-07-03", 16)
	dev := must(svc.RegisterAE(ctx, uDev, "Dev (as an AE)", "dev@ual.li"))
	must(svc.SaveAEProfile(ctx, dev.ID, acc.AEProfile{Name: dev.Name, Email: dev.Email, Countries: []string{"UY"}, PayoutMethod: acc.PayoutTransfer, PayoutDetails: "Banco Ejemplo, cuenta de prueba (demo)"}))
	must(svc.AuthorizeAE(ctx, dev.ID, uStaff))

	// --- what the platform reports: the customers and what they are billed ---
	m, d := strconv.FormatInt(marta.ID, 10), strconv.FormatInt(diego.ID, 10)
	pf.tenants = []acc.TenantSnapshot{
		{ExternalRef: "constructec", Name: "Constructec S.A.", Country: "UY", AERef: m, Account: "Constructec", Registration: "2026-06-18"},
		{ExternalRef: "frigocold", Name: "FrigoCold S.A.", Country: "UY", AERef: m, Account: "FrigoCold", Registration: "2026-06-22"},
		{ExternalRef: "extinfire", Name: "Extinfire SRL", Country: "UY", AERef: d, Account: "Extinfire", Registration: "2026-06-25"},
		{ExternalRef: "vendiberia", Name: "VendiBeria S.L.", Country: "ES", AERef: d, Account: "VendiBeria", Registration: "2026-06-29"},
		{ExternalRef: "fairborne", Name: "Fairborne Equipment LLC", Country: "US", AERef: acc.SlugSeamAE, Account: "Fairborne", Registration: "2026-06-30"},
		{ExternalRef: "asahikenki", Name: "Asahi Kenki K.K.", Country: "JP", AERef: acc.SlugSeamWeb, Account: "Asahi Kenki", Registration: "2026-07-01"},
		{ExternalRef: sandbox, Name: "Sandbox Demo Co.", Country: "UY", AERef: strconv.FormatInt(dev.ID, 10), Account: "Sandbox", Registration: "2026-07-02"},
	}
	monthly := []struct{ tenant, amount string }{
		{"constructec", "480.00"}, {"frigocold", "620.00"}, {"extinfire", "350.00"}, {"vendiberia", "540.00"}, {"fairborne", "900.00"}, {"asahikenki", "760.00"},
	}
	months := []string{"2026-07", "2026-08", "2026-09", "2026-10", "2026-11"}
	item := func(tenant, period string, c acc.Concept, desc, amount string) string {
		ref := fmt.Sprintf("%s-%s-%s", tenant, period, c)
		pf.items = append(pf.items, acc.ItemSnapshot{ExternalRef: ref, TenantRef: tenant, Period: period, Concept: c, Description: desc, Amount: usd(amount), State: acc.ItemPending})
		return ref
	}
	for _, p := range months {
		for _, t := range monthly {
			item(t.tenant, p, acc.ConceptSubscription, "Seam AMS subscription, "+p, t.amount)
		}
		if p >= "2026-09" {
			item("constructec-py", p, acc.ConceptSubscription, "Seam AMS subscription, "+p, "240.00")
		}
		item(sandbox, p, acc.ConceptSubscription, "Seam AMS subscription, "+p, "100.00")
	}
	item("fairborne", "2026-07", acc.ConceptStartup, "Onboarding and data load", "1200.00")
	item("vendiberia", "2026-07", acc.ConceptStartup, "Onboarding and data load", "800.00")
	item("constructec-py", "2026-09", acc.ConceptStartup, "Onboarding of the Paraguay branch", "300.00")
	item("frigocold", "2026-09", acc.ConceptAddon, "Cold-chain sensor pack, 20 probes", "180.00")
	item("vendiberia", "2026-10", acc.ConceptServices, "On-site training, two days", "650.00")

	itemID := func(ref string) int64 {
		for _, it := range must(svc.ListItems(ctx, acc.ItemFilter{Limit: 2000})) {
			if it.ExternalRef == ref {
				return it.ID
			}
		}
		check(fmt.Errorf("no item %s", ref))
		return 0
	}
	cash := []string{acc.CodeBank, acc.CodePayPal, acc.CodeDLocalGo}
	n := 0
	collect := func(day, ref string) {
		at(day, 10+n%6)
		must(svc.MarkCollected(ctx, itemID(ref), cash[n%3], day, uStaff))
		n++
	}
	collectMonth := func(period string, first int, skip ...string) {
		day := first
		for _, it := range pf.items {
			ref := it.ExternalRef
			if it.Period != period || contains(skip, ref) || it.TenantRef == sandbox {
				continue
			}
			collect(fmt.Sprintf("%s-%02d", period, day), ref)
			day += 2
		}
	}
	// The sandbox tenant is collected apart, at a fixed hour into the bank, so
	// that the story of the others reads the same with or without it.
	collectSandbox := func(day, period string) {
		at(day, 17)
		must(svc.MarkCollected(ctx, itemID(fmt.Sprintf("%s-%s-%s", sandbox, period, acc.ConceptSubscription)), acc.CodeBank, day, uStaff))
	}
	sync := func(day, month string) {
		at(day, 9)
		pf.month = month
		res := must(svc.Sync(ctx))
		if len(res.Problems) > 0 {
			check(fmt.Errorf("sync %s: %s", day, strings.Join(res.Problems, "; ")))
		}
	}
	settle := func(day string, ae acc.AE, actor int64, number string) acc.Settlement {
		at(day, 15)
		lines := must(svc.UnsettledLines(ctx, ae.ID))
		var ids []int64
		var total acc.Cents
		for _, l := range lines {
			ids = append(ids, l.ID)
			total += l.Amount
		}
		return must(svc.CreateSettlement(ctx, ae.ID, ids, acc.DocumentInput{Type: acc.DocInvoice, Number: number, Date: day, Filename: "invoice-" + number + ".pdf", Data: pdf, AmountUSD: total}, actor))
	}
	pay := func(day string, st acc.Settlement, number string) {
		at(day, 16)
		_, _, err := svc.PaySettlement(ctx, st.ID, acc.DocumentInput{Type: acc.DocPaymentProof, Number: number, Date: day, Filename: "payment-" + number + ".pdf", Data: pdf, AmountUSD: st.Total}, day, uStaff)
		check(err)
	}
	post := func(day, desc string, amount string, debit, credit string, doc *acc.DocumentInput) acc.Entry {
		at(day, 12)
		a := usd(amount)
		return must(svc.PostEntryWithDocument(ctx, acc.Entry{Date: day, Description: desc, Source: acc.SourceManual,
			Movements: []acc.Movement{{AccountCode: debit, Side: acc.Debit, Amount: a}, {AccountCode: credit, Side: acc.Credit, Amount: a}}}, doc, uAccountant))
	}
	closeMonth := func(day, period string) {
		at(day, 18)
		must(svc.CloseMonth(ctx, period, uAccountant))
	}

	// --- July ---
	sync("2026-07-05", "2026-07")
	collectMonth("2026-07", 8)
	post("2026-07-06", "Aporte inicial de capital", "5000.00", acc.CodeBank, "3.1.1", nil)
	collectSandbox("2026-07-28", "2026-07")

	// --- August: July is closed, the first settlements are paid ---
	closeMonth("2026-08-03", "2026-07")
	sync("2026-08-04", "2026-08")
	collectMonth("2026-08", 6, "extinfire-2026-08-subscription")
	pay("2026-08-15", settle("2026-08-12", marta, uMarta, "A-0041"), "TRF-88213")
	pay("2026-08-18", settle("2026-08-14", diego, uDiego, "0007"), "PP-5KX81")
	pay("2026-08-22", settle("2026-08-21", dev, uDev, "D-0001"), "TRF-88990")
	collectSandbox("2026-08-27", "2026-08")
	post("2026-08-20", "Comisiones bancarias de agosto", "18.50", acc.CodeCollectExp, acc.CodeBank,
		&acc.DocumentInput{Type: acc.DocReceipt, Number: "BK-2026-08", Date: "2026-08-20", Filename: "bank-fees-2026-08.pdf", Data: pdf, AmountUSD: usd("18.50"), ForEntry: true})

	// --- September: a late collection, a new tenant, an entry made by mistake and reversed ---
	closeMonth("2026-09-02", "2026-08")
	pf.tenants = append(pf.tenants, acc.TenantSnapshot{ExternalRef: "constructec-py", Name: "Constructec Paraguay S.A.", Country: "PY", AERef: m, Account: "Constructec", Registration: "2026-08-28"})
	sync("2026-09-03", "2026-09")
	collect("2026-09-04", "extinfire-2026-08-subscription")
	collectMonth("2026-09", 7)
	post("2026-09-15", "Hosting y servicios en la nube, septiembre", "320.00", acc.CodeOperatingEx, acc.CodePayPal,
		&acc.DocumentInput{Type: acc.DocInvoice, Number: "INV-77120", Date: "2026-09-15", Filename: "hosting-2026-09.pdf", Data: pdf, AmountUSD: usd("320.00"), ForEntry: true,
			Currency: "EUR", ForeignTotal: "297,60", FxRate: "1,0753", FxDate: "2026-09-15"})
	wrong := post("2026-09-16", "Tasa municipal (cargada por error)", "150.00", acc.CodeTaxesExp, acc.CodeBank, nil)
	at("2026-09-17", 9)
	must(svc.ReverseEntry(ctx, wrong.ID, "2026-09-17", uAccountant))
	pay("2026-09-24", settle("2026-09-20", marta, uMarta, "A-0047"), "TRF-90377")
	collectSandbox("2026-09-28", "2026-09")

	// --- October: a settlement waits for its payment, a third AE registers ---
	closeMonth("2026-10-02", "2026-09")
	sync("2026-10-03", "2026-10")
	collectMonth("2026-10", 6, "asahikenki-2026-10-subscription")
	settle("2026-10-20", diego, uDiego, "0011")
	collectSandbox("2026-10-27", "2026-10")
	at("2026-10-28", 14)
	must(svc.RegisterAE(ctx, uPaula, "Paula Méndez", "paula.mendez@example.com"))

	// --- November: the open month ---
	closeMonth("2026-11-03", "2026-10")
	pf.inProgress["frigocold-2026-11-subscription"] = true
	pf.inProgress["vendiberia-2026-11-subscription"] = true
	sync("2026-11-04", "2026-11")
	collect("2026-11-09", "constructec-2026-11-subscription")
	collect("2026-11-10", "fairborne-2026-11-subscription")
	collect("2026-11-12", "constructec-py-2026-11-subscription")
	collect("2026-11-13", "asahikenki-2026-10-subscription")

	write(ctx, store)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// writing the .seed file
// ---------------------------------------------------------------------------

// Fields that hold the id of another record, and the entity it is in.
var refs = map[string]string{
	"ae_id": acc.EntAE, "tenant_id": acc.EntTenant, "item_id": acc.EntItem, "instance_id": acc.EntInstance,
	"line_id": acc.EntCommission, "line_ids": acc.EntCommission, "settlement_id": acc.EntSettlement,
	"invoice_doc_id": acc.EntDocument, "payment_doc_id": acc.EntDocument, "doc_id": acc.EntDocument,
}

// Fields that hold the id of a user.
var userRefs = map[string]bool{"user_id": true, "created_by": true, "collected_by": true, "paid_by": true, "closed_by": true,
	"authorized_by": true, "uploaded_by": true, "updated_by": true}

var short = map[string]string{
	acc.EntAE: "ae", acc.EntAccount: "account", acc.EntEntry: "entry", acc.EntMovement: "movement", acc.EntInstance: "instance",
	acc.EntTenant: "tenant", acc.EntItem: "item", acc.EntCommission: "line", acc.EntSettlement: "settlement", acc.EntDocument: "doc",
	acc.EntClose: "close", acc.EntCompany: "company", acc.EntFormula: "formula",
}

var (
	refText      = regexp.MustCompile(`^(item|settlement|entry):(\d+)$`)
	reversalText = regexp.MustCompile(`^(Reversal of entry #)(\d+)(: .*)$`)
	textEntity   = map[string]string{"item": acc.EntItem, "settlement": acc.EntSettlement, "entry": acc.EntEntry}
)

func name(entity string, id int64) string { return fmt.Sprintf("%s_%d", short[entity], id) }

func bind(entity string, v any) any {
	id := int64(v.(float64))
	if id == 0 {
		return v
	}
	return map[string]any{"$seed": name(entity, id)}
}

// stable holds, for every random identifier the Service made (the uid of an
// entry, the key of a document's file), the one written in its place: a digest
// of the record's position, so that running the generator again writes the
// same file.
var stable = map[string]string{}

func stabilize(kind string, n int, random string) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("seed-accounting/%s/%d", kind, n)))
	stable[random] = hex.EncodeToString(sum[:12])
}

func rewrite(entity string, data map[string]any) map[string]any {
	out := map[string]any{}
	users := map[int64]string{}
	for _, p := range people {
		users[p.id] = p.as
	}
	for k, v := range data {
		switch {
		case userRefs[k]:
			if as, ok := users[int64(v.(float64))]; ok {
				v = map[string]any{"$seed": as}
			} else if v.(float64) != 0 {
				check(fmt.Errorf("%s.%s holds a user id nobody in the seed has: %v", entity, k, v))
			}
		case k == "line_ids":
			list := []any{}
			for _, x := range v.([]any) {
				list = append(list, bind(refs[k], x))
			}
			v = list
		case refs[k] != "":
			v = bind(refs[k], v)
		case k == "reference" && entity == acc.EntEntry:
			if m := refText.FindStringSubmatch(v.(string)); m != nil {
				id, _ := strconv.ParseInt(m[2], 10, 64)
				v = map[string]any{"$text": fmt.Sprintf("%s:${seed:%s}", m[1], name(textEntity[m[1]], id))}
			}
		case k == "description" && entity == acc.EntEntry:
			if m := reversalText.FindStringSubmatch(v.(string)); m != nil {
				id, _ := strconv.ParseInt(m[2], 10, 64)
				v = map[string]any{"$text": fmt.Sprintf("%s${seed:%s}%s", m[1], name(acc.EntEntry, id), m[3])}
			}
		}
		if text, ok := v.(string); ok && stable[text] != "" {
			v = stable[text]
		}
		out[k] = v
	}
	return out
}

func line(v any) string {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	check(enc.Encode(v))
	// json.Marshal writes compact JSON; the files of this repository put a space after ":" and ","
	return spaced(strings.TrimRight(sb.String(), "\n"))
}

// spaced adds the space after ':' and ',' outside strings.
func spaced(s string) string {
	var sb strings.Builder
	inString, escaped := false, false
	for _, r := range s {
		sb.WriteRune(r)
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case !inString && (r == ':' || r == ','):
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

// record writes one record: on one line when it is short, otherwise with one
// field of its data per continuation line.
func record(w *bufio.Writer, head map[string]any, bodyKey string, body map[string]any) {
	whole := map[string]any{}
	for k, v := range head {
		whole[k] = v
	}
	whole[bodyKey] = body
	one := ordered(whole, bodyKey)
	if len(one) <= 110 {
		fmt.Fprintln(w, one)
		return
	}
	h := ordered(head, "")
	fmt.Fprintf(w, "%s, %s: {\n", h[:len(h)-1], line(bodyKey))
	keys := sortedKeys(body)
	for i, k := range keys {
		sep := ","
		if i == len(keys)-1 {
			sep = "}}"
		}
		fmt.Fprintf(w, "  %s: %s%s\n", line(k), line(body[k]), sep)
	}
}

// ordered writes an object with "kind" first and the body last, as the other seed files do.
func ordered(m map[string]any, last string) string {
	first := []string{"seed", "kind", "entity", "method", "path", "match", "as"}
	var parts []string
	done := map[string]bool{}
	for _, k := range first {
		if v, ok := m[k]; ok && k != last {
			parts = append(parts, line(k)+": "+line(v))
			done[k] = true
		}
	}
	for _, k := range sortedKeys(m) {
		if !done[k] && k != last {
			parts = append(parts, line(k)+": "+line(m[k]))
		}
	}
	if v, ok := m[last]; ok {
		parts = append(parts, line(last)+": "+line(v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func write(ctx context.Context, store *acc.MemStore) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	all := func(entity string) []acc.Doc {
		docs := must(store.Find(ctx, acc.Query{Entity: entity, Limit: 100000}))
		sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
		return docs
	}
	// what a record is found by
	match := func(entity string, data map[string]any) []string {
		switch entity {
		case acc.EntInstance:
			return []string{"name"}
		case acc.EntAccount:
			return []string{"code"}
		case acc.EntCompany:
			return []string{"legal_name"}
		case acc.EntFormula:
			return []string{"situation", "name"}
		case acc.EntAE:
			if data["kind"] == string(acc.AEVirtual) {
				return []string{"slug"}
			}
			return []string{"email"}
		case acc.EntTenant, acc.EntItem:
			return []string{"external_ref"}
		case acc.EntDocument:
			return []string{"blob_key"}
		case acc.EntCommission:
			return []string{"item_id"}
		case acc.EntSettlement:
			return []string{"invoice_doc_id"}
		case acc.EntEntry:
			return []string{"uid"}
		case acc.EntMovement:
			return []string{"entry_uid", "account_code", "side", "amount"}
		case acc.EntClose:
			return []string{"period"}
		}
		check(fmt.Errorf("no match rule for %s", entity))
		return nil
	}
	order := []struct{ entity, about string }{
		{acc.EntInstance, "The platform the module reads tenants and items from. The module makes this record itself when it starts."},
		{acc.EntAE, "Account Executives: the two virtual ones, two authorized and one that registered and waits."},
		{acc.EntAccount, "The chart of accounts: the one the module makes when it starts, and the account of each authorized AE, which names it."},
		{acc.EntCompany, "The company the AEs invoice."},
		{acc.EntFormula, "Formula rules: what an AE earns on a collected item, and price adjustments by market."},
		{acc.EntTenant, "The customers, as the platform reported them. Each belongs to one AE."},
		{acc.EntItem, "Billable items, July to November 2026. November is the open month: some are pending or in progress."},
		{acc.EntDocument, "Invoices, payment proofs and receipts. Only the records: the files are kept by Seam on disk and are not in a seed."},
		{acc.EntCommission, "Commission lines: one for each collected item of an AE with a formula."},
		{acc.EntSettlement, "Settlements: three paid, one with its invoice loaded and waiting for payment."},
		{acc.EntEntry, "The journal: collections, commissions, settlements paid and the entries made by hand (one reversed)."},
		{acc.EntMovement, "The movements of the journal entries."},
		{acc.EntClose, "Monthly closes: July to October 2026."},
	}
	for i, d := range all(acc.EntEntry) {
		stabilize("entry", i, d.Data["uid"].(string))
	}
	for i, d := range all(acc.EntDocument) {
		stabilize("document", i, d.Data["blob_key"].(string))
	}
	entities := []string{"users"}
	counts := map[string]int{}
	for _, o := range order {
		entities = append(entities, o.entity)
		counts[o.entity] = len(all(o.entity))
	}
	entities = append(entities, acc.EntClaim)

	fmt.Fprintln(w, "# Seam AMS demonstration data: the Accounting & Admin module (docs/ACCOUNTING_ADMIN_MODULE.md).")
	fmt.Fprintln(w, "# A .seed file, format version 2 (docs/SEED_FORMAT.md). It is an addition: it loads over any seed, or alone:")
	fmt.Fprintln(w, "#   python3 seeds/seed.py load accounting --url http://localhost:9090")
	fmt.Fprintln(w, "# Written by cmd/accounting-seedgen, which runs the module's own rules over five months of 2026; do not edit it by")
	fmt.Fprintln(w, "# hand: change the generator and run it again. The dates of the books are fixed, they do not move with the load.")
	logins := []any{}
	for _, p := range people {
		logins = append(logins, map[string]any{"email": p.email, "password": "holaquetal", "role": p.role})
	}
	header := map[string]any{"seed": 2, "name": "accounting", "version": "1", "addition": true, "tags": []string{"accounting"},
		"description": "Accounting & Admin: AEs, tenants, collections, commissions, settlements and the books, July to November 2026",
		"entities":    entities,
		"notes": fmt.Sprintf("%d AEs, %d tenants, %d items, %d journal entries, %d settlements, %d monthly closes. Seam must run with the module on (SEAM_ENABLE_ACCOUNTING=1).",
			counts[acc.EntAE], counts[acc.EntTenant], counts[acc.EntItem], counts[acc.EntEntry], counts[acc.EntSettlement], counts[acc.EntClose]),
		"x": map[string]any{"seam": map[string]any{"module": "accounting", "logins": logins}}}
	h := "{" + line("seed") + ": 2"
	for _, k := range []string{"name", "description", "tags", "version", "addition", "entities", "notes", "x"} {
		h += ",\n  " + line(k) + ": " + line(header[k])
	}
	fmt.Fprintln(w, h+"}")

	fmt.Fprintln(w, "\n# The people of the module: back office, accountant, three AEs, and the developer account, which is an AE too so that a developer can view the portal as one. The password of all of them is in the header.")
	for _, p := range people {
		record(w, map[string]any{"kind": "entity", "entity": "users", "match": []string{"email"}, "as": p.as}, "data", map[string]any{
			"active": true, "created_at": map[string]any{"$now": ""}, "updated_at": map[string]any{"$now": ""}, "email": p.email, "name": p.name,
			"password_hash": passwordHash, "role": p.role, "tenant_id": 1})
	}
	for _, o := range order {
		fmt.Fprintf(w, "\n# %s\n", o.about)
		seen := map[string]bool{}
		for _, d := range all(o.entity) {
			data := rewrite(o.entity, d.Data)
			keys := match(o.entity, data)
			var id []string
			for _, k := range keys {
				id = append(id, line(d.Data[k]))
			}
			if key := strings.Join(id, "|"); seen[key] {
				check(fmt.Errorf("two %s records have the same %v: %s", o.entity, keys, key))
			} else {
				seen[key] = true
			}
			record(w, map[string]any{"kind": "entity", "entity": o.entity, "match": keys, "as": name(o.entity, d.ID)}, "data", data)
		}
	}
	fmt.Fprintln(w, "\n# The claim of each settled commission line. Its id is the id of the line, which is what makes \"a line is paid")
	fmt.Fprintln(w, "# once\" a rule of the store, so these are requests with the id in the path.")
	for _, d := range all(acc.EntClaim) {
		record(w, map[string]any{"kind": "call", "method": "POST", "path": fmt.Sprintf("/api/v1/%s/save/${%s}", acc.EntClaim, name(acc.EntCommission, d.ID))},
			"body", rewrite(acc.EntClaim, d.Data))
	}
}
