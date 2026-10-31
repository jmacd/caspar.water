import json
import os
import re
import sys

from bankstatementparser.hybrid import (
    IngestResult,
    LLMExtractor,
    VerificationStatus,
    extract_text_pages,
    ollama_direct_completion,
    verify_transactions,
)
from bankstatementparser._llm_common import parse_json_payload
from datetime import datetime, timezone
from importlib.metadata import version
from urllib.parse import quote, urlparse


STATEMENT_PERIOD_INSTRUCTION = """

Also extract the explicit statement cycle dates. Add this top-level field
to the JSON object:
  "statement_period": {
    "start": "YYYY-MM-DD",
    "end": "YYYY-MM-DD"
  }
Use the dates printed for the statement period, beginning balance, and
ending balance. Do not substitute the first or last transaction date.
If the statement cycle dates are not explicit, return null.
"""


path, endpoint, api_version, model = sys.argv[1:]
url = urlparse(endpoint)
if (
    url.scheme not in ("http", "https")
    or not url.hostname
    or url.username
    or url.password
    or url.query
    or url.fragment
):
    raise ValueError("statement.api_base must be an HTTP(S) API base URL")
provider, separator, provider_model = model.partition("/")
if not separator or not provider_model or provider not in ("ollama", "azure"):
    raise ValueError("statement.model must use an ollama/ or azure/ provider prefix")
if provider == "azure":
    azure_hosts = (".openai.azure.com", ".cognitiveservices.azure.com")
    if (
        url.scheme != "https"
        or not any(url.hostname.endswith(suffix) for suffix in azure_hosts)
        or url.path not in ("", "/")
    ):
        raise ValueError(
            "Azure statement.api_base must use an https://*.openai.azure.com "
            "or https://*.cognitiveservices.azure.com base URL"
        )
    if not api_version:
        raise ValueError("statement.api_version is required for Azure")
    if not re.fullmatch(r"[A-Za-z0-9._-]+", provider_model):
        raise ValueError("Azure statement.model deployment contains unsupported characters")
    azure_token = os.environ.get("AZURE_AD_TOKEN", "")
    if not azure_token:
        raise ValueError("AZURE_AD_TOKEN is required for Azure statement extraction")


def completion(**kwargs):
    messages = [dict(message) for message in kwargs["messages"]]
    messages[0]["content"] += STATEMENT_PERIOD_INSTRUCTION
    kwargs["messages"] = messages
    if provider == "ollama":
        kwargs["timeout"] = 1100
        return ollama_direct_completion(**kwargs)
    import httpx

    response = httpx.post(
        endpoint.rstrip("/")
        + "/openai/deployments/"
        + quote(provider_model, safe="")
        + "/chat/completions",
        params={"api-version": api_version},
        headers={"Authorization": "Bearer " + azure_token},
        json={"messages": kwargs["messages"]},
        timeout=300,
    )
    response.raise_for_status()
    return response.json()


layout = "\n".join(extract_text_pages(path))
extractor = LLMExtractor(
    model=model,
    api_base=endpoint,
    completion_fn=completion,
)
text_result = extractor.extract(layout)
verification = verify_transactions(
    text_result.transactions,
    opening_balance=text_result.opening_balance,
    closing_balance=text_result.closing_balance,
)
if verification.status != VerificationStatus.VERIFIED:
    raise ValueError(
        f"bank statement text extraction did not balance-verify: {verification.message}"
    )
payload = parse_json_payload(text_result.raw_response)
period = payload.get("statement_period")
if not isinstance(period, dict):
    raise ValueError("statement extraction did not identify its explicit statement period")
if set(period) != {"start", "end"}:
    raise ValueError("statement period must contain only start and end dates")
start, end = period["start"], period["end"]
if (
    not isinstance(start, str)
    or not isinstance(end, str)
    or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", start)
    or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", end)
):
    raise ValueError("statement period requires YYYY-MM-DD start and end dates")
try:
    start_date = datetime.strptime(start, "%Y-%m-%d").date()
    end_date = datetime.strptime(end, "%Y-%m-%d").date()
except ValueError as exc:
    raise ValueError("statement period requires valid calendar dates") from exc
if end_date < start_date:
    raise ValueError("statement period dates are out of order")
result = IngestResult(
    source_method="llm",
    source_format="pdf",
    transactions=tuple(text_result.transactions),
    verification=verification,
)
document = json.loads(result.to_json())
document["wat_statement_period"] = period
document["wat_extraction"] = {
    "parser_version": version("bankstatementparser"),
    "provider": provider,
    "api_base": endpoint,
    "api_version": api_version or None,
    "model": model,
    "created_at": datetime.now(timezone.utc).isoformat(),
    "source_method": result.source_method,
}
print(json.dumps(document))
