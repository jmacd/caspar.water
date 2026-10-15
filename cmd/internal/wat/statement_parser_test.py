import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import subprocess
import sys
from tempfile import TemporaryDirectory
from threading import Thread
import unittest

from pypdf import PdfWriter
from pypdf.generic import DictionaryObject, NameObject, DecodedStreamObject


class ParserBridgeTest(unittest.TestCase):
    def run_bridge(self, *, pages, reason="stop"):
        seen = []
        payload = {
            "account_id": "SYNTHETIC",
            "currency": "USD",
            "opening_balance": "0.00",
            "closing_balance": "100.00",
            "transactions": [{
                "booking_date": "2021-11-10",
                "description": "DEPOSIT",
                "amount": "100.00",
            }],
        }

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                seen.append(request)
                self.send_response(200)
                self.send_header("Content-Type", "application/x-ndjson")
                self.end_headers()
                for data in [
                    {"message": {"content": json.dumps(payload)}, "done": False},
                    {"done": True, "done_reason": reason},
                ]:
                    self.wfile.write((json.dumps(data) + "\n").encode())

            def log_message(self, *_):
                pass

        server = HTTPServer(("127.0.0.1", 0), Handler)
        thread = Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with TemporaryDirectory() as temporary:
                pdf = Path(temporary) / "test.pdf"
                writer = PdfWriter()
                page = writer.add_blank_page(width=612, height=792)
                font = DictionaryObject({
                    NameObject("/Type"): NameObject("/Font"),
                    NameObject("/Subtype"): NameObject("/Type1"),
                    NameObject("/BaseFont"): NameObject("/Helvetica"),
                })
                page[NameObject("/Resources")] = DictionaryObject({
                    NameObject("/Font"): DictionaryObject({NameObject("/F1"): font})
                })
                stream = DecodedStreamObject()
                stream.set_data(
                    b"BT /F1 12 Tf 50 700 Td "
                    b"(Synthetic bank statement: 2021-11-10 DEPOSIT USD 100.00. Opening 0.00 Closing 100.00.) Tj ET"
                )
                page[NameObject("/Contents")] = stream
                writer.add_blank_page(width=612, height=792)
                with pdf.open("wb") as output:
                    writer.write(output)
                result = subprocess.run(
                    [sys.executable, "-c", Path(__file__).with_name("statement_parser.py").read_text(),
                     str(pdf), f"http://127.0.0.1:{server.server_port}", "test-model", json.dumps(pages)],
                    text=True, capture_output=True, timeout=30,
                )
                return result, seen
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)

    def test_selected_pages_streaming_and_provenance(self):
        result, requests = self.run_bridge(pages=[1])
        self.assertEqual(result.returncode, 0, result.stderr)
        document = json.loads(result.stdout)
        self.assertEqual(document["transactions"][0]["amount"], "100.00")
        self.assertEqual(document["verification"]["status"], "verified")
        self.assertEqual(document["selected_pages"], [1])
        self.assertIn("excluded PDF pages [2]", " ".join(document["warnings"]))
        self.assertEqual(document["wat_extraction"]["model"], "test-model")
        self.assertFalse(requests[0]["think"])
        self.assertEqual(requests[0]["format"], "json")

    def test_blank_pages_require_explicit_selection(self):
        result, requests = self.run_bridge(pages=None)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("lack a usable text layer", result.stderr)
        self.assertEqual(requests, [])

    def test_truncated_output_is_not_accepted(self):
        result, _ = self.run_bridge(pages=[1], reason="length")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("truncated", result.stderr)
        self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main()
