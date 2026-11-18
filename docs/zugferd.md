# ZUGFeRD / XRechnung invoices

## The terms

- **E-invoice**: an invoice a computer can read. German B2B invoices have to be electronic (receiving them since 2025, sending them from 2027/2028).
- **EN 16931**: the EU standard that says which fields an e-invoice has (seller, buyer, lines, VAT, totals, ...).
- **XRechnung**: Germany's stricter version of EN 16931. It adds required fields such as a buyer reference, a seller contact with phone and email, and payment details.
- **CII XML**: the XML format the invoice data is written in.
- **PDF/A-3**: a PDF variant for long-term archiving. All fonts and colour profiles live inside the file, and other files can be attached to it.
- **ZUGFeRD**: a PDF/A-3 that people can read like a normal invoice, with the XML attached so software can read it too. The profile we use is `XRECHNUNG`, so the attachment is named `xrechnung.xml`.

## The flow

1. **Build the invoice data** as an `einvoice.Invoice`: seller, buyer, lines, payment details.
2. **`invoice.Finalize`** marks it as XRechnung 3.0, works out the VAT per rate and the totals from the lines, and validates it. If a required field is missing, it returns an error (e.g. `BR-DE-15`: buyer reference missing).
3. **`invoice.XML`** writes the invoice as CII XML.
4. **`invoice.WritePDF`** fills `template.html` with the same invoice data and renders it to a PDF. The visible PDF and the XML come from the same data, so they can't disagree.

## How the PDF becomes PDF/A-3

We use [`bagme`](https://github.com/boxesandglue/bagme), which turns HTML/CSS into a PDF. Passing `document.WithZUGFeRD(xml, "XRECHNUNG")` handles the PDF/A part:

| PDF/A-3 needs | Done by |
|-|-|
| All fonts embedded | bagme always embeds its built-in fonts (`font-family: sans`) |
| A colour profile (sRGB) | added automatically for PDF/A |
| PDF/A declared in the file's metadata (XMP) | set by `WithZUGFeRD` |
| The XML attached and marked as belonging to the invoice | `WithZUGFeRD` attaches `xrechnung.xml` |
| ZUGFeRD metadata (file name, profile) | `WithZUGFeRD` adds it |

Libraries like maroto or gofpdf **can't** do this. They don't embed the colour profile or write the PDF/A metadata.

## Try it

```sh
go run ./cmd/zugferd-poc -xml out/invoice.xml   # writes out/invoice.pdf (out/ is gitignored)
go test ./...
```

## Checking the output

- **PDF/A**: `docker run --rm -v "$PWD":/data verapdf/cli --flavour 3b /data/invoice.pdf` should print `PASS`.
- **Full ZUGFeRD + XRechnung check**: use the [Mustang CLI](https://github.com/ZUGFeRD/mustangproject/releases), `java -jar Mustang-CLI.jar --action validate --source invoice.pdf`.
- `einvoice`'s own `Validate()` re-implements the rules in Go. Before going live, also check invoices with an official validator (KoSIT or Mustang).
