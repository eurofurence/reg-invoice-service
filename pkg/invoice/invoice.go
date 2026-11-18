// Package invoice builds XRechnung invoices and renders them as ZUGFeRD PDFs
// (PDF/A-3b with the CII XML embedded as xrechnung.xml).
//
// The einvoice.Invoice is the single source of truth: the XML is serialized
// from it and the visible PDF is rendered from it, so both always agree.
package invoice

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/speedata/einvoice"
)

// Finalize stamps inv as an XRechnung 3.0 CII invoice, derives the VAT
// breakdown and document totals from the invoice lines, and validates it
// against EN 16931 and the German BR-DE rules.
func Finalize(inv *einvoice.Invoice) error {
	inv.GuidelineSpecifiedDocumentContextParameter = einvoice.SpecXRechnung30
	inv.SchemaType = einvoice.CII

	inv.UpdateApplicableTradeTax(nil)
	inv.UpdateTotals()

	if err := inv.Validate(); err != nil {
		var valErr *einvoice.ValidationError
		if !errors.As(err, &valErr) {
			return err
		}
		msgs := make([]string, 0, len(valErr.Violations()))
		for _, v := range valErr.Violations() {
			msgs = append(msgs, fmt.Sprintf("%s: %s", v.Rule.Code, v.Text))
		}
		return fmt.Errorf("invoice is not a valid XRechnung:\n  %s", strings.Join(msgs, "\n  "))
	}
	return nil
}

// XML serializes inv as CII XML.
func XML(inv *einvoice.Invoice) ([]byte, error) {
	var buf bytes.Buffer
	if err := inv.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
