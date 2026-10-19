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
from datetime import datetime, timezone
from importlib.metadata import version
from urllib.parse import quote, urlparse


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
period = None
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
result = IngestResult(
    source_method="llm",
    source_format="pdf",
    transactions=tuple(text_result.transactions),
    verification=verification,
)
document = json.loads(result.to_json())
if period is not None:
    document["wat_statement_period"] = period
else:
    document["warnings"].append(
        "Statement period unavailable: no unambiguous dated Beginning Balance and Ending Balance labels found in the PDF."
    )
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
