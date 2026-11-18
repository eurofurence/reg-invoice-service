package invoice

import (
	"bytes"
	_ "embed"
	"html/template"
	"io"
	"os"
	"strings"
	"time"

	"github.com/boxesandglue/bagme/document"
	pdfdoc "github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/frontend"
	"github.com/shopspring/decimal"
	"github.com/speedata/einvoice"
)

//go:embed template.html
var templateHTML string

//go:embed template.css
var templateCSS string

var tmpl = template.Must(template.New("invoice").Funcs(template.FuncMap{
	"money": formatMoney,
	"qty":   formatQuantity,
	"pct":   formatPercent,
	"date":  formatDate,
}).Parse(templateHTML))

// WritePDF renders inv as a ZUGFeRD PDF to the file at path. See RenderPDF.
func WritePDF(inv *einvoice.Invoice, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := RenderPDF(inv, f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// PDF renders inv as a ZUGFeRD PDF and returns its bytes. See RenderPDF.
func PDF(inv *einvoice.Invoice) ([]byte, error) {
	var buf bytes.Buffer
	if err := RenderPDF(inv, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderPDF renders inv as a ZUGFeRD (profile XRECHNUNG) PDF to w. The
// invoice must have been passed through Finalize.
func RenderPDF(inv *einvoice.Invoice, w io.Writer) error {
	xml, err := XML(inv)
	if err != nil {
		return err
	}

	var html bytes.Buffer
	if err := tmpl.Execute(&html, inv); err != nil {
		return err
	}

	fe, err := frontend.NewForWriter(w)
	if err != nil {
		return err
	}
	if err := setupZUGFeRD(fe.Doc, xml); err != nil {
		return err
	}
	d, err := document.NewWithFrontend(fe)
	if err != nil {
		return err
	}
	d.Title = "Rechnung " + inv.InvoiceNumber
	d.Author = inv.Seller.Name
	d.Subject = "Rechnung " + inv.InvoiceNumber + " an " + inv.Buyer.Name
	d.Creator = "reg-invoice-service"
	d.Language = "de"

	if err := d.AddCSS(templateCSS); err != nil {
		return err
	}
	if err := d.RenderPages(html.String()); err != nil {
		return err
	}
	return d.Finish()
}

// setupZUGFeRD makes doc a PDF/A-3b with xml attached as xrechnung.xml. It
// does what bagme's document.WithZUGFeRD(xml, "XRECHNUNG") does, which is
// only available for document.New writing to a file path.
func setupZUGFeRD(doc *pdfdoc.PDFDocument, xml []byte) error {
	doc.Format = pdfdoc.FormatPDFA3b
	// HTML/CSS colors are RGB, so PDF/A needs the sRGB output intent.
	if _, err := doc.LoadSRGBColorprofile(); err != nil {
		return err
	}
	doc.AttachFile(pdfdoc.Attachment{
		Name:           "xrechnung.xml",
		Description:    "XRechnung invoice",
		MimeType:       "text/xml",
		Data:           xml,
		AFRelationship: "Alternative",
	})
	doc.AddXMPExtension(pdfdoc.XMPExtension{
		Schema:       "Factur-X PDFA Extension Schema",
		NamespaceURI: "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#",
		Prefix:       "fx",
		Properties: []pdfdoc.XMPExtensionProperty{
			{Name: "DocumentFileName", ValueType: "Text", Category: "external", Description: "The name of the embedded XML document"},
			{Name: "DocumentType", ValueType: "Text", Category: "external", Description: "The type of the hybrid document in capital letters, e.g. INVOICE or ORDER"},
			{Name: "Version", ValueType: "Text", Category: "external", Description: "The actual version of the standard applying to the embedded XML document"},
			{Name: "ConformanceLevel", ValueType: "Text", Category: "external", Description: "The conformance level of the embedded XML document"},
		},
		Values: map[string]string{
			"DocumentFileName": "xrechnung.xml",
			"DocumentType":     "INVOICE",
			"Version":          "1.0",
			"ConformanceLevel": "XRECHNUNG",
		},
	})
	return nil
}

// formatMoney formats d German style with two decimals, e.g. 1234.5 -> "1.234,50".
func formatMoney(d decimal.Decimal) string {
	s := d.StringFixed(2)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, _ := strings.Cut(s, ".")

	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	out := b.String() + "," + frac
	if neg {
		out = "-" + out
	}
	return out
}

func formatQuantity(d decimal.Decimal) string {
	return strings.Replace(d.String(), ".", ",", 1)
}

func formatPercent(d decimal.Decimal) string {
	return strings.Replace(d.String(), ".", ",", 1) + " %"
}

func formatDate(t time.Time) string {
	return t.Format("02.01.2006")
}
