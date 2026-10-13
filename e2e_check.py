"""Run the same HTTP flow against a panel (Go or Python) and dump results.

Usage: python e2e_check.py <base_url> <output_json>

The output stores RAW response bodies (scrubbed of timestamps) so the Go and
Python panels can be diffed byte-for-byte, key order included.
"""
import json
import re
import sys
import urllib.request

BASE = sys.argv[1]
OUT = sys.argv[2]

raw = {}


def scrub_text(s):
    # timestamps inside strings (paper lines, ISO datetimes)
    s = re.sub(r"\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}", "<DT>", s)
    s = re.sub(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[\d.:]*Z?", "<DT>", s)
    # volatile json fields, wherever they appear in the body
    s = re.sub(r'"(?:ts|saved_at)":\s*"[^"]*",?\s*', "", s)
    return s


def fetch(path, payload=None):
    if payload is None:
        req = urllib.request.Request(BASE + path)
    else:
        req = urllib.request.Request(
            BASE + path,
            data=json.dumps(payload).encode(),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
    try:
        with urllib.request.urlopen(req) as r:
            return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()


def record(name, path, payload=None):
    """Call an endpoint once, keep its raw scrubbed body, return parsed JSON."""
    status, body = fetch(path, payload)
    raw[name] = "%d\n%s" % (status, scrub_text(body))
    if status != 200:
        return {"__status": status}
    return json.loads(body)


# 1. program company
company = {
    "rif": "J-12345678-9",
    "razon_social": "EMPRESA DE PRUEBA C.A.",
    "direccion": "Av. Principal, Caracas",
    "municipio": "Libertador",
    "estado": "Distrito Capital",
    "telefono": "0212-5555555",
    "representante": "JUAN PEREZ",
}
co = record("company", "/api/company", {"company": company})

# 2. invoice via raw TFHKA commands
seq = [
    "!   2.000     50.00HARINA 1KG MAIZ",
    "!   1.000    120.00ACEITE VEGETAL 1L",
    "3",
    "103cash",
    "100   220.00",
    "101",
]
cmds = [record("cmd%d" % i, "/api/command", {"command": c})
        for i, c in enumerate(seq)]

# 3. X and Z reports
rep_x = record("report_x", "/api/report", {"type": "X"})
rep_z = record("report_z", "/api/report", {"type": "Z"})

# 4. state
st = record("state", "/api/state")

# 5. preview + jobs + history + company endpoints
prev = record("preview", "/api/preview")
jobs = record("jobs", "/api/jobs")
hist = record("history", "/api/history?since=0")
co_get = record("company_get", "/api/company")

# 6. 404 contract
nf = record("not_found", "/api/nope")

# ---- sanity assertions -------------------------------------------------
c = st["counters"]
assert c["invoices"] == 1, c
assert c["x_reports"] == 1, c
assert c["z_reports"] == 1, c
assert "open" in prev and "lines" in prev, prev
assert jobs["jobs"], "no jobs"
job0 = record("job0", "/api/job/%d" % jobs["jobs"][0]["id"])
assert len(job0["lines"]) >= 10, job0
assert hist["count"] >= 7, hist["count"]
assert raw["not_found"].startswith("404"), raw["not_found"][:80]
assert co["company"]["rif"] == "J-12345678-9", co

with open(OUT, "w", encoding="utf-8") as f:
    json.dump(raw, f, ensure_ascii=False, indent=1, sort_keys=True)

print("E2E OK ->", OUT)
