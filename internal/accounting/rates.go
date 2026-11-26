package accounting

import "context"

// RateCalculator is the built-in, deliberately simple commission formula: a
// percentage (in basis points, 100 = 1%) of the amount collected, per kind of
// AE. All rates default to zero, which means no commission: the design leaves
// the real formula outside the schema, and a made-up rate would book
// liabilities that do not exist. Anything richer goes in a CommissionCalculator
// of its own (WithCalculator).
type RateCalculator struct {
	RealBPS    int // AEs registered by a person
	SeamAEBPS  int // the virtual AE "Seam AE"
	SeamWebBPS int // the virtual AE "Seam Web"
}

// Enabled reports whether any rate is above zero.
func (r RateCalculator) Enabled() bool { return r.RealBPS > 0 || r.SeamAEBPS > 0 || r.SeamWebBPS > 0 }

func (r RateCalculator) bps(ae AE) int {
	if ae.Kind == AEVirtual {
		switch ae.Slug {
		case SlugSeamAE:
			return r.SeamAEBPS
		case SlugSeamWeb:
			return r.SeamWebBPS
		}
		return 0
	}
	return r.RealBPS
}

// Calculate returns one line: amount * rate, rounded half up to the cent.
func (r RateCalculator) Calculate(_ context.Context, item Item, _ Tenant, ae AE) ([]CommissionResult, error) {
	bps := r.bps(ae)
	if bps <= 0 || item.Amount <= 0 {
		return nil, nil
	}
	amt := (item.Amount*Cents(bps) + 5000) / 10000
	if amt <= 0 {
		return nil, nil
	}
	return []CommissionResult{{Detail: bpsLabel(bps), Amount: amt}}, nil
}

func bpsLabel(bps int) string {
	whole, frac := bps/100, bps%100
	if frac == 0 {
		return itoaInt(whole) + "%"
	}
	f := itoaInt(frac)
	if frac < 10 {
		f = "0" + f
	}
	f = trimRight0(f)
	return itoaInt(whole) + "." + f + "%"
}

func itoaInt(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

func trimRight0(s string) string {
	for len(s) > 1 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	return s
}
