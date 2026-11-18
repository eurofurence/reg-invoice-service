// Command zugferd-poc writes a sample XRechnung invoice as a ZUGFeRD PDF.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/shopspring/decimal"
	"github.com/speedata/einvoice"

	"github.com/eurofurence/reg-invoice-service/pkg/invoice"
)

func main() {
	out := flag.String("out", "out/invoice.pdf", "path of the ZUGFeRD PDF to write")
	xmlOut := flag.String("xml", "", "optionally also write the embedded XRechnung XML to this path")
	number := flag.String("number", "RE-2026-0001", "invoice number")
	flag.Parse()

	for _, p := range []string{*out, *xmlOut} {
		if p == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			log.Fatal(err)
		}
	}

	inv := sampleInvoice(*number, time.Now())
	if err := invoice.Finalize(inv); err != nil {
		log.Fatal(err)
	}

	if *xmlOut != "" {
		xml, err := invoice.XML(inv)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*xmlOut, xml, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s", *xmlOut)
	}

	if err := invoice.WritePDF(inv, *out); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (XRechnung 3.0, %s %s)", *out, inv.GrandTotal.StringFixed(2), inv.InvoiceCurrencyCode)
}

// sampleInvoice returns an invoice with fictitious parties that carries every
// field XRechnung 3.0 requires. Totals and the VAT breakdown are left for
// invoice.Finalize to compute.
func sampleInvoice(number string, issued time.Time) *einvoice.Invoice {
	issued = issued.Truncate(24 * time.Hour)
	return &einvoice.Invoice{
		InvoiceNumber:       number,
		InvoiceTypeCode:     380, // commercial invoice
		InvoiceDate:         issued,
		OccurrenceDateTime:  issued,
		InvoiceCurrencyCode: "EUR",
		BuyerReference:      "REG-4711", // Leitweg-ID for public buyers, otherwise any buyer reference
		PaymentReference:    number,
		Notes: []einvoice.Note{{
			Text: "Vielen Dank für Ihre Anmeldung zur Beispiel-Convention 2026.",
		}},
		Seller: einvoice.Party{
			Name:                            "Beispiel Convention e.V.",
			VATaxRegistration:               "DE123456789",
			URIUniversalCommunication:       "billing@example.org",
			URIUniversalCommunicationScheme: "EM",
			PostalAddress: &einvoice.PostalAddress{
				Line1:        "Musterstraße 1",
				PostcodeCode: "12345",
				City:         "Musterstadt",
				CountryID:    "DE",
			},
			DefinedTradeContact: []einvoice.DefinedTradeContact{{
				PersonName:  "Registration Team",
				PhoneNumber: "+49 30 1234567",
				EMail:       "billing@example.org",
			}},
		},
		Buyer: einvoice.Party{
			Name:                            "Erika Mustermann",
			URIUniversalCommunication:       "erika@example.com",
			URIUniversalCommunicationScheme: "EM",
			PostalAddress: &einvoice.PostalAddress{
				Line1:        "Beispielweg 42",
				PostcodeCode: "54321",
				City:         "Beispielstadt",
				CountryID:    "DE",
			},
		},
		PaymentMeans: []einvoice.PaymentMeans{{
			TypeCode:                               58, // SEPA credit transfer
			PayeePartyCreditorFinancialAccountIBAN: "DE89370400440532013000",
			PayeePartyCreditorFinancialAccountName: "Beispiel Convention e.V.",
			PayeeSpecifiedCreditorFinancialInstitutionBIC: "COBADEFFXXX",
		}},
		SpecifiedTradePaymentTerms: []einvoice.SpecifiedTradePaymentTerms{{
			DueDate: issued.AddDate(0, 0, 14),
		}},
		InvoiceLines: []einvoice.InvoiceLine{
			line("1", "Convention-Ticket Sponsor", "Teilnahme 4 Tage", 1, "120.00", "19"),
			line("2", "Convention T-Shirt", "Größe L", 2, "18.50", "19"),
			line("3", "Programmheft", "", 1, "5.00", "7"),
		},
	}
}

func line(id, name, desc string, qty int64, netPrice, vatPercent string) einvoice.InvoiceLine {
	q := decimal.NewFromInt(qty)
	price := decimal.RequireFromString(netPrice)
	return einvoice.InvoiceLine{
		LineID:                   id,
		ItemName:                 name,
		Description:              desc,
		BilledQuantity:           q,
		BilledQuantityUnit:       "C62", // "one" (piece)
		NetPrice:                 price,
		Total:                    q.Mul(price),
		TaxTypeCode:              "VAT",
		TaxCategoryCode:          "S",
		TaxRateApplicablePercent: decimal.RequireFromString(vatPercent),
	}
}
