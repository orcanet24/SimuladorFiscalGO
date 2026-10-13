#!/usr/bin/env python3
"""Verifica que el panel es responsiv: sin overflow horizontal, sin secciones
recortadas y con los breakpoints correctos, en una bateria de viewports.

Uso: python tools/responsive_check.py [--base http://127.0.0.1:8082]
"""
import argparse
import sys

ap = argparse.ArgumentParser()
ap.add_argument("--base", default="http://127.0.0.1:8082")
args = ap.parse_args()

VIEWPORTS = [
    (1920, 1080, "desktop-full"),
    (1536, 864, "laptop-15-125pct"),
    (1366, 768, "laptop-15-100pct"),
    (1280, 800, "laptop-ancho-min"),
    (1024, 768, "tablet-landscape"),
    (768, 1024, "tablet-portrait"),
    (390, 844, "movil"),
]

JS_PROBE = """() => {
  const d = document.documentElement;
  const header = document.querySelector('header');
  const pill = document.querySelector('.status-pill');
  const tabs = document.querySelector('.brand-tabs');
  const main = getComputedStyle(document.querySelector('main'));
  const g3 = document.querySelector('.grid3');
  const g3cs = g3 ? getComputedStyle(g3).gridTemplateColumns.split(' ').length : 0;
  const cards = document.querySelector('.cards');
  const cardCols = getComputedStyle(cards).gridTemplateColumns.split(' ').length;
  const clipped = [...document.querySelectorAll('.section')]
    .filter(s => s.scrollHeight > s.clientHeight + 2)
    .map(s => s.querySelector('h3').textContent.trim().slice(0, 24));
  const pillOwnRow = pill.getBoundingClientRect().top >= tabs.getBoundingClientRect().bottom - 1;
  const narrowInputs = [...document.querySelectorAll('.section input, .section select')]
    .filter(i => i.offsetParent !== null && i.type !== 'checkbox' && i.type !== 'radio')
    .filter(i => i.getBoundingClientRect().width < 110)
    .length;
  return {
    hOverflow: d.scrollWidth - innerWidth,
    headerOverflow: header.scrollWidth - header.clientWidth,
    mainCols: main.gridTemplateColumns,
    g3Cols: g3cs, cardCols,
    clipped, pillOwnRow, narrowInputs,
    controlScroll: (() => { const c = document.querySelector('.control-side');
      return c.scrollHeight > c.clientHeight; })(),
  };
}"""


def main():
    from playwright.sync_api import sync_playwright

    failures = []
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        for w, h, name in VIEWPORTS:
            page.set_viewport_size({"width": w, "height": h})
            page.goto(args.base, wait_until="networkidle")
            page.wait_for_timeout(500)
            r = page.evaluate(JS_PROBE)
            bad = []
            if r["hOverflow"] > 0:
                bad.append("hOverflow=%d" % r["hOverflow"])
            if r["headerOverflow"] > 0:
                bad.append("headerOverflow=%d" % r["headerOverflow"])
            if r["clipped"]:
                bad.append("clipped=%s" % r["clipped"])
            if r["narrowInputs"]:
                bad.append("narrowInputs=%d" % r["narrowInputs"])
            # header a dos filas desde <=1100
            expect_pill_row = w <= 1100
            if expect_pill_row and not r["pillOwnRow"]:
                bad.append("pill-no-own-row")
            status = "FAIL " + ", ".join(bad) if bad else "PASS"
            print("%-20s %4dx%-4d %s | main=%s cards=%d g3=%d scroll=%s pillRow=%s"
                  % (name, w, h, status, r["mainCols"],
                     r["cardCols"], r["g3Cols"], r["controlScroll"], r["pillOwnRow"]))
            if bad:
                failures.append((name, bad))
        browser.close()

    if failures:
        print("\n%d viewports con problemas" % len(failures))
        sys.exit(1)
    print("\nTodos los viewports OK")


if __name__ == "__main__":
    main()
