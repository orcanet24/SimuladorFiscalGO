#!/usr/bin/env python3
"""Genera las capturas de pantalla del README de la version Go (img/).

Uso:
    ./simuladorfiscal.exe panel --no-browser --port 8091   # en otra terminal
    python tools/shots.py [--base http://127.0.0.1:8091]

Requiere: playwright (pip install playwright && playwright install chromium)
"""
import argparse
import json
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
IMG = ROOT / "img"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://127.0.0.1:8091")
    args = ap.parse_args()
    base = args.base.rstrip("/")
    IMG.mkdir(exist_ok=True)

    # La consola Windows usa cp1252; evita UnicodeEncodeError con "→"
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    def api(path, payload=None):
        if payload is None:
            req = urllib.request.Request(base + path)
        else:
            req = urllib.request.Request(
                base + path,
                data=json.dumps(payload).encode(),
                headers={"Content-Type": "application/json"},
            )
        with urllib.request.urlopen(req, timeout=10) as r:
            return json.load(r)

    def seed():
        """Estado limpio y demostrativo: memoria fiscal + 2 facturas + 1 doc en curso."""
        api("/api/company", {"brand": "tfhka", "company": {
            "rif": "J-31055823-7",
            "razon_social": "DISTRIBUIDORA ANDELINA, C.A.",
            "direccion": "AV. PRINCIPAL DE LAS MERCEDES, EDIF. TORRE AZUL, PISO 4, OFIC. 401",
            "municipio": "CHACAO",
            "ciudad": "CARACAS",
            "estado": "DISTRITO CAPITAL",
            "telefono": "0212-9512204",
            "email": "ventas@andelina.com.ve",
            "representante_legal": "MARIA ELENA RODRIGUEZ SUAREZ",
            "cedula_representante": "V-10234567",
            "actividad_economica": "COMERCIO AL POR MENOR DE PRODUCTOS ALIMENTICIOS Y BEBIDAS",
            "moneda": "Bs.",
            "condiciones_pago": "CONTADO / CRÉDITO",
            "contribuyente_especial": True,
            "agente_retencion": True,
        }})
        # Factura 1 (tarjeta)
        api("/api/sale", {"brand": "tfhka", "action": "open", "params": {}})
        api("/api/sale", {"brand": "tfhka", "action": "item",
                          "params": {"description": "CAFE MOLIDO 500G", "qty": 2,
                                     "price": 3.9, "tax": "general"}})
        api("/api/sale", {"brand": "tfhka", "action": "payment",
                          "params": {"amount": 9.02, "method": "card"}})
        api("/api/sale", {"brand": "tfhka", "action": "close", "params": {}})
        # Factura 2 (cliente + 4 ítems) — queda PAGADA, sin cerrar,
        # para que la captura principal muestre la vista previa en vivo.
        api("/api/sale", {"brand": "tfhka", "action": "customer",
                          "params": {"rif": "J-50123456-1",
                                     "name": "COMERCIAL EL COMUNERO, C.A.",
                                     "address": "CALLE REAL DE CHACAO, NRO 15-23"}})
        api("/api/sale", {"brand": "tfhka", "action": "open", "params": {}})
        for item in (
            {"description": "ARROZ VENEZOLANO 1KG", "qty": 6, "price": 2.5, "tax": "general"},
            {"description": "ACEITE VEGETAL 1L", "qty": 3, "price": 4.9, "tax": "general"},
            {"description": "LECHE EN POLVO 1KG", "qty": 2, "price": 7.5, "tax": "reduced"},
            {"description": "PAN CASERO", "qty": 4, "price": 1.2, "tax": "exempt"},
        ):
            api("/api/sale", {"brand": "tfhka", "action": "item", "params": item})
        api("/api/sale", {"brand": "tfhka", "action": "subtotal", "params": {}})
        api("/api/sale", {"brand": "tfhka", "action": "payment",
                          "params": {"amount": 66.35, "method": "cash"}})

    from playwright.sync_api import sync_playwright

    # Para regenerar todo desde cero: borra data/state_*.json y vuelve a ejecutar.
    st = api("/api/state?brand=tfhka")
    if st["command_count"] == 0:
        print("Estado vacío: sembrando demo...")
        seed()
    else:
        print("Estado existente (no se siembra); borra data/state_*.json para regenerar.")

    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page(viewport={"width": 1600, "height": 1000},
                                device_scale_factor=2)
        page.goto(base, wait_until="networkidle")
        page.wait_for_timeout(1200)

        # 1) Panel principal con documento en curso (vista previa en vivo)
        page.screenshot(path=str(IMG / "panel-principal.png"))
        print("img/panel-principal.png")

        # 2) Memoria fiscal (formulario con los datos de la empresa)
        mem = page.locator("div.section", has_text="Memoria fiscal").first
        mem.scroll_into_view_if_needed()
        page.wait_for_timeout(300)
        mem.screenshot(path=str(IMG / "memoria-fiscal.png"))
        print("img/memoria-fiscal.png")

        # Cerrar el documento para generar el ticket impreso
        api("/api/sale", {"brand": "tfhka", "action": "close", "params": {}})
        page.wait_for_timeout(1000)

        paper = page.locator("aside.paper-side")
        sel = page.locator("#jobSelect")

        def shot_paper(name):
            jobs = api("/api/jobs?brand=tfhka")["jobs"]
            if jobs:
                sel.select_option(value=str(jobs[0]["id"]))
                page.wait_for_timeout(600)
            paper.screenshot(path=str(IMG / name))
            print("img/" + name, "job=", jobs[0]["title"] if jobs else "live")

        # 3) Ticket / factura impresa en el papel virtual
        shot_paper("factura-fiscal.png")

        # 4) Reporte X
        page.locator("button", has_text="Reporte X").click()
        page.wait_for_timeout(1200)
        shot_paper("reporte-x.png")

        # 5) Reporte Z (cierre del día)
        page.locator("button", has_text="Reporte Z").click()
        page.wait_for_timeout(1200)
        shot_paper("reporte-z.png")
        z = api("/api/state?brand=tfhka")
        print("   post-Z: ventas dia=", z["daily"]["sales"],
              "historicas=", z["totals"]["lifetime_sales"])

        # 6) Registro de actividad + consola de comandos
        log_sec = page.locator("div.section", has_text="Registro de actividad").first
        log_sec.scroll_into_view_if_needed()
        page.wait_for_timeout(400)
        log_sec.screenshot(path=str(IMG / "registro-actividad.png"))
        print("img/registro-actividad.png")

        browser.close()


if __name__ == "__main__":
    main()
