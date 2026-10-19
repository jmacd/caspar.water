import json
from http.server import BaseHTTPRequestHandler, HTTPServer
import os
from pathlib import Path
import subprocess
import sys
from tempfile import TemporaryDirectory
from threading import Thread
import unittest

from pypdf import PdfWriter
from pypdf.generic import DictionaryObject, NameObject, DecodedStreamObject


class ParserBridgeTest(unittest.TestCase):
    def run_bridge(
        self,
        *,
        dates="Beginning Balance 11/01/21 Ending Balance 11/30/21",
        closing_balance="100.00",
        statement_period=None,
        provider="ollama",
        azure_token="synthetic-token",
        azure_host="synthetic.cognitiveservices.azure.com",
    ):
        seen = []
        if statement_period is None:
            statement_period = {"start": "2021-11-01", "end": "2021-11-30"}
        payload = {
            "account_id": "SYNTHETIC",
            "currency": "USD",
            "opening_balance": "0.00",
            "closing_balance": closing_balance,
            "statement_period": statement_period,
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
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps({
                    "message": {"content": json.dumps(payload)},
                }).encode())

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
                text = (
                    "Synthetic statement: 2021-11-10 DEPOSIT USD 100.00. "
                    "Opening 0.00 Closing 100.00. " + dates
                )
                stream.set_data(("BT /F1 12 Tf 50 700 Td (" + text + ") Tj ET").encode())
                page[NameObject("/Contents")] = stream
                writer.add_blank_page(width=612, height=792)
                with pdf.open("wb") as output:
                    writer.write(output)
                endpoint = f"http://127.0.0.1:{server.server_port}"
                api_version = ""
                model = "ollama/test-model"
                env = os.environ.copy()
                if provider == "azure":
                    endpoint = f"https://{azure_host}/"
                    api_version = "2025-04-01-preview"
                    model = "azure/test-deployment"
                    env["AZURE_AD_TOKEN"] = azure_token
                    seen_path = Path(temporary) / "seen.json"
                    env["WAT_TEST_SEEN"] = str(seen_path)
                    fake = Path(temporary) / "httpx.py"
                    fake.write_text(
                        "import json, os\n"
                        f"_payload = {payload!r}\n"
                        "class Response:\n"
                        "    def raise_for_status(self): pass\n"
                        "    def json(self):\n"
                        "        return {'choices': [{'message': {'content': json.dumps(_payload)}}]}\n"
                        "def post(url, **kwargs):\n"
                        "    visible = {key: value for key, value in kwargs.items() if key != 'json'}\n"
                        "    visible['url'] = url\n"
                        "    visible['message_count'] = len(kwargs['json']['messages'])\n"
                        "    with open(os.environ['WAT_TEST_SEEN'], 'w') as output:\n"
                        "        json.dump(visible, output)\n"
                        "    return Response()\n"
                    )
                    env["PYTHONPATH"] = temporary
                result = subprocess.run(
                    [sys.executable, "-c", Path(__file__).with_name("statement_parser.py").read_text(),
                     str(pdf), endpoint, api_version, model],
                    text=True, capture_output=True, timeout=30, env=env,
                )
                if provider == "azure" and seen_path.exists():
                    seen.append(json.loads(seen_path.read_text()))
                return result, seen
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)

    def test_complete_pdf_text_and_provenance(self):
        result, requests = self.run_bridge()
        self.assertEqual(result.returncode, 0, result.stderr)
        document = json.loads(result.stdout)
        self.assertEqual(document["transactions"][0]["amount"], "100.00")
        self.assertEqual(document["verification"]["status"], "verified")
        self.assertEqual(document["wat_statement_period"], {"start": "2021-11-01", "end": "2021-11-30"})
        self.assertEqual(document["wat_extraction"]["provider"], "ollama")
        self.assertEqual(document["wat_extraction"]["model"], "ollama/test-model")
        self.assertEqual(document["wat_extraction"]["source_method"], "llm")
        self.assertFalse(requests[0]["stream"])
        self.assertNotIn("images", requests[0]["messages"][1])
        self.assertIn("statement_period", requests[0]["messages"][0]["content"])

    def test_azure_uses_entra_token_with_direct_chat_completion(self):
        result, requests = self.run_bridge(provider="azure")
        self.assertEqual(result.returncode, 0, result.stderr)
        document = json.loads(result.stdout)
        self.assertEqual(document["wat_extraction"]["provider"], "azure")
        self.assertEqual(document["wat_extraction"]["model"], "azure/test-deployment")
        self.assertEqual(len(requests), 1)
        self.assertEqual(
            requests[0]["url"],
            "https://synthetic.cognitiveservices.azure.com/openai/deployments/test-deployment/chat/completions",
        )
        self.assertEqual(requests[0]["params"], {"api-version": "2025-04-01-preview"})
        self.assertEqual(requests[0]["headers"]["Authorization"], "Bearer synthetic-token")
        self.assertEqual(requests[0]["message_count"], 2)

    def test_azure_accepts_legacy_openai_endpoint(self):
        result, requests = self.run_bridge(
            provider="azure",
            azure_host="synthetic.openai.azure.com",
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(requests), 1)
        self.assertEqual(
            requests[0]["url"],
            "https://synthetic.openai.azure.com/openai/deployments/test-deployment/chat/completions",
        )

    def test_azure_rejects_lookalike_endpoint(self):
        result, requests = self.run_bridge(
            provider="azure",
            azure_host="synthetic.cognitiveservices.azure.com.example.test",
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Azure statement.api_base must use", result.stderr)
        self.assertEqual(requests, [])

    def test_azure_requires_entra_token(self):
        result, requests = self.run_bridge(provider="azure", azure_token="")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("AZURE_AD_TOKEN is required", result.stderr)
        self.assertEqual(requests, [])

    def test_balance_discrepancy_is_rejected(self):
        result, requests = self.run_bridge(closing_balance="101.00")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("did not balance-verify", result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertEqual(len(requests), 1)

    def test_model_period_does_not_depend_on_transaction_bounds(self):
        result, _ = self.run_bridge(
            dates="",
            statement_period={"start": "2021-11-01", "end": "2021-11-30"},
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        document = json.loads(result.stdout)
        self.assertEqual(
            document["wat_statement_period"],
            {"start": "2021-11-01", "end": "2021-11-30"},
        )

    def test_missing_statement_period_is_rejected(self):
        result, requests = self.run_bridge(statement_period=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("did not identify its explicit statement period", result.stderr)
        self.assertEqual(len(requests), 1)

    def test_reversed_period_is_rejected(self):
        result, requests = self.run_bridge(
            statement_period={"start": "2021-12-01", "end": "2021-11-30"},
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("period dates are out of order", result.stderr)
        self.assertEqual(len(requests), 1)


if __name__ == "__main__":
    unittest.main()
