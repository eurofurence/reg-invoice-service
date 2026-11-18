package invoice

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/shopspring/decimal"
	"github.com/speedata/einvoice"
)

func testInvoice() *einvoice.Invoice {
	date := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	address := func(street, zip, city string) *einvoice.PostalAddress {
		return &einvoice.PostalAddress{Line1: street, PostcodeCode: zip, City: city, CountryID: "DE"}
	}
	line := func(id, price, vat string) einvoice.InvoiceLine {
		p := decimal.RequireFromString(price)
		return einvoice.InvoiceLine{
			LineID: id, ItemName: "Item " + id,
			BilledQuantity: decimal.NewFromInt(1), BilledQuantityUnit: "C62",
			NetPrice: p, Total: p,
			TaxTypeCode: "VAT", TaxCategoryCode: "S", TaxRateApplicablePercent: decimal.RequireFromString(vat),
		}
	}
	return &einvoice.Invoice{
		InvoiceNumber:       "T-1",
		InvoiceTypeCode:     380,
		InvoiceDate:         date,
		InvoiceCurrencyCode: "EUR",
		BuyerReference:      "REF-1",
		Seller: einvoice.Party{
			Name:                            "Seller e.V.",
			VATaxRegistration:               "DE123456789",
			URIUniversalCommunication:       "seller@example.org",
			URIUniversalCommunicationScheme: "EM",
			PostalAddress:                   address("Street 1", "12345", "Town"),
			DefinedTradeContact: []einvoice.DefinedTradeContact{{
				PersonName: "Team", PhoneNumber: "+49 30 1234567", EMail: "seller@example.org",
			}},
		},
		Buyer: einvoice.Party{
			Name:                            "Buyer",
			URIUniversalCommunication:       "buyer@example.com",
			URIUniversalCommunicationScheme: "EM",
			PostalAddress:                   address("Road 2", "54321", "City"),
		},
		PaymentMeans: []einvoice.PaymentMeans{{
			TypeCode:                               58,
			PayeePartyCreditorFinancialAccountIBAN: "DE89370400440532013000",
		}},
		SpecifiedTradePaymentTerms: []einvoice.SpecifiedTradePaymentTerms{{DueDate: date.AddDate(0, 0, 14)}},
		InvoiceLines:               []einvoice.InvoiceLine{line("1", "100.00", "19"), line("2", "10.00", "7")},
	}
}

func TestFinalize(t *testing.T) {
	t.Run("accepts a complete invoice as XRechnung 3.0", func(t *testing.T) {
		inv := testInvoice()
		if err := Finalize(inv); err != nil {
			t.Fatal(err)
		}
		if !inv.IsXRechnung() {
			t.Errorf("spec identifier = %q, want XRechnung", inv.GuidelineSpecifiedDocumentContextParameter)
		}
	})

	t.Run("computes VAT per rate and the grand total from the lines", func(t *testing.T) {
		inv := testInvoice()
		if err := Finalize(inv); err != nil {
			t.Fatal(err)
		}
		if len(inv.TradeTaxes) != 2 {
			t.Fatalf("got %d VAT breakdowns, want 2", len(inv.TradeTaxes))
		}
		// 100 * 19% + 10 * 7% = 19.70 VAT on 110.00 net
		if want := decimal.RequireFromString("129.70"); !inv.GrandTotal.Equal(want) {
			t.Errorf("grand total = %s, want %s", inv.GrandTotal, want)
		}
	})

	t.Run("rejects an invoice without buyer reference", func(t *testing.T) {
		inv := testInvoice()
		inv.BuyerReference = ""
		err := Finalize(inv)
		if err == nil || !strings.Contains(err.Error(), "BR-DE-15") {
			t.Errorf("got %v, want BR-DE-15 violation", err)
		}
	})
}

func TestPDF(t *testing.T) {
	t.Run("embeds the invoice XML as xrechnung.xml", func(t *testing.T) {
		inv := testInvoice()
		if err := Finalize(inv); err != nil {
			t.Fatal(err)
		}
		pdf, err := PDF(inv)
		if err != nil {
			t.Fatal(err)
		}

		attachments, err := api.ExtractAttachmentsRaw(context.Background(), bytes.NewReader(pdf), "", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(attachments) != 1 || attachments[0].FileName != "xrechnung.xml" {
			t.Fatalf("got attachments %v, want exactly xrechnung.xml", attachments)
		}
		embedded, err := io.ReadAll(attachments[0])
		if err != nil {
			t.Fatal(err)
		}

		parsed, err := einvoice.ParseReader(bytes.NewReader(embedded))
		if err != nil {
			t.Fatal(err)
		}
		if err := parsed.Validate(); err != nil {
			t.Errorf("embedded XML does not validate: %v", err)
		}
		if parsed.InvoiceNumber != inv.InvoiceNumber {
			t.Errorf("embedded invoice number = %q, want %q", parsed.InvoiceNumber, inv.InvoiceNumber)
		}
	})
}
