package state

import (
	"simuladorfiscalgo/internal/pyfmt"
)

// PrintJobMeta is the JSON shape served by GET /api/jobs: the metadata
// without the paper lines.
type PrintJobMeta struct {
	ID    int      `json:"id"`
	Kind  string   `json:"kind"`
	Title string   `json:"title"`
	TS    string   `json:"ts"`
	Meta  *JobMeta `json:"meta"`
}

// JobMeta carries the metadata attached to a print job. Field order matches
// the Python reference dicts exactly:
//
//	receipt: {"number", "total", "doc_type"}
//	report:  {"number", "total_sales", "total_tax"}
type JobMeta struct {
	Number     int          `json:"number"`
	Total      *pyfmt.PyFloat `json:"total,omitempty"`
	TotalSales *pyfmt.PyFloat `json:"total_sales,omitempty"`
	TotalTax   *pyfmt.PyFloat `json:"total_tax,omitempty"`
	DocType    string       `json:"doc_type,omitempty"`
}

// NewReceiptMeta builds the metadata for a closed receipt. The total is
// rounded to 2 decimals, like Python's round(doc["total"], 2).
func NewReceiptMeta(number int, total float64, docType string) *JobMeta {
	t := pyfmt.PyFloat(pyfmt.PyRound(total, 2))
	return &JobMeta{Number: number, Total: &t, DocType: docType}
}

// NewReportMeta builds the metadata for an X/Z report, rounding both totals
// to 2 decimals exactly like the Python reference does.
func NewReportMeta(number int, totalSales, totalTax float64) *JobMeta {
	s := pyfmt.PyFloat(pyfmt.PyRound(totalSales, 2))
	x := pyfmt.PyFloat(pyfmt.PyRound(totalTax, 2))
	return &JobMeta{Number: number, TotalSales: &s, TotalTax: &x}
}
