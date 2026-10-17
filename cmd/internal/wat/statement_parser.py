import json
import re
import sys
import time

import httpx
from bankstatementparser.hybrid import LLMExtractor, smart_ingest
from pypdf import PdfReader, PdfWriter
from pathlib import Path
from tempfile import TemporaryDirectory
from urllib.parse import urlparse
from importlib.metadata import version
from datetime import datetime, timezone


path, endpoint, model, selected = sys.argv[1:]
url = urlparse(endpoint)
if url.scheme not in ("http", "https") or not url.hostname or url.username or url.password or url.query or url.fragment:
    raise ValueError("statement.ollama_url must be an HTTP(S) Ollama base URL")
model = model.removeprefix("ollama/")
reader = PdfReader(path)
pages = json.loads(selected) or list(range(1, len(reader.pages) + 1))
if len(set(pages)) != len(pages) or any(p < 1 or p > len(reader.pages) for p in pages):
    raise ValueError("--statement-pages must contain unique, valid 1-based page numbers")
sparse = [p for p in pages if len((reader.pages[p - 1].extract_text() or "").strip()) < 50]
if sparse:
    raise ValueError(
        f"PDF pages {sparse} lack a usable text layer. Inspect them before explicitly "
        "selecting transaction pages with --statement-pages. This version does not OCR scans."
    )

period = None
layout = "\n".join(reader.pages[p - 1].extract_text(extraction_mode="layout") or "" for p in pages)
date_pattern = r"(\d{1,2}/\d{1,2}/(?:\d{4}|\d{2}))(?=\s|$)"
beginning = set(re.findall(r"Beginning\s+Balance\s+" + date_pattern, layout, re.IGNORECASE))
ending = set(re.findall(r"Ending\s+Balance\s+" + date_pattern, layout, re.IGNORECASE))
if len(beginning) == 1 and len(ending) == 1:
    def iso_date(text):
        date_format = "%m/%d/%Y" if len(text.rsplit("/", 1)[1]) == 4 else "%m/%d/%y"
        return datetime.strptime(text, date_format).date().isoformat()

    start, end = iso_date(next(iter(beginning))), iso_date(next(iter(ending)))
    if end < start:
        raise ValueError("PDF beginning/ending balance dates are out of order")
    period = {"start": start, "end": end}


def completion(**kwargs):
    payload = {
        "model": model,
        "messages": kwargs["messages"],
        "stream": True,
        "format": "json",
        "think": False,
        "options": {"temperature": 0.0, "num_ctx": 16384, "num_predict": 8192},
    }
    chunks = []
    finished = False
    started = time.monotonic()
    with httpx.Client(timeout=httpx.Timeout(1100.0, connect=10.0), trust_env=False) as client:
        with client.stream("POST", endpoint.rstrip("/") + "/api/chat", json=payload) as reply:
            reply.raise_for_status()
            for line in reply.iter_lines():
                if not line:
                    continue
                data = json.loads(line)
                if data.get("error"):
                    raise RuntimeError(data["error"])
                if time.monotonic() - started > 1100:
                    raise TimeoutError("statement extraction exceeded its time limit")
                text = data.get("message", {}).get("content", "")
                if text:
                    chunks.append(text)
                    if len(chunks) == 1 or len(chunks) % 250 == 0:
                        print(f"Ollama: {len(chunks)} response chunks received", file=sys.stderr, flush=True)
                if data.get("done"):
                    if data.get("done_reason") == "length":
                        raise RuntimeError("Ollama response was truncated; extract fewer transaction pages")
                    finished = True
    if not finished:
        raise RuntimeError("Ollama stream ended without a completion marker")
    return {"choices": [{"message": {"content": "".join(chunks)}}]}


with TemporaryDirectory(prefix="wat-statement-") as temporary:
    extracted_pdf = Path(temporary) / "selected.pdf"
    writer = PdfWriter()
    for page in pages:
        writer.add_page(reader.pages[page - 1])
    with extracted_pdf.open("wb") as output:
        writer.write(output)
    result = smart_ingest(
        extracted_pdf,
        extractor=LLMExtractor(
            model="ollama/" + model, api_base=endpoint, completion_fn=completion
        ),
    )
    document = json.loads(result.to_json())
    if period is not None:
        document["wat_statement_period"] = period
    else:
        document["warnings"].append(
            "Statement period unavailable: no unambiguous dated Beginning Balance and Ending Balance labels found in selected PDF pages."
        )
    document["selected_pages"] = pages
    document["wat_extraction"] = {
        "parser_version": version("bankstatementparser"),
        "ollama_url": endpoint,
        "model": model,
        "created_at": datetime.now(timezone.utc).isoformat(),
        "thinking": False,
    }
    excluded = [p for p in range(1, len(reader.pages) + 1) if p not in pages]
    if excluded:
        document["warnings"].append(f"Operator explicitly excluded PDF pages {excluded}")
    print(json.dumps(document))
