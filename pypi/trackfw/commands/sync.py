"""
commands/sync.py — Subcomando `trackfw sync`.
Sincroniza REQs Open para ferramentas de PM (Linear, Jira).
Espelho Python de npm/src/commands/sync.js.

Nota: requer HTTPS para APIs externas (Linear/Jira). Se as credenciais não
estiverem configuradas, o comando imprime mensagem orientativa e sai com erro.
"""

import json
import os
import sys
import urllib.request
import urllib.error
import base64


# ---------------------------------------------------------------------------
# Config helpers
# ---------------------------------------------------------------------------

def _read_config_field(field):
    """Lê campo do trackfw.yaml sem dependências externas."""
    try:
        with open("trackfw.yaml", "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return ""
    prefix = field + ":"
    for line in content.split("\n"):
        trimmed = line.strip()
        if trimmed.startswith(prefix):
            value = trimmed[len(prefix):].strip()
            if len(value) >= 2 and value[0] in ('"', "'") and value[-1] == value[0]:
                value = value[1:-1]
            return value
    return ""


def _get_config(field, env_var):
    return _read_config_field(field) or os.environ.get(env_var, "")


# ---------------------------------------------------------------------------
# HTTP helper
# ---------------------------------------------------------------------------

def _http_request(url, method="GET", headers=None, body=None):
    """
    Faz uma requisição HTTP/HTTPS simples.
    Retorna (status_code, body_str).
    """
    data = body.encode("utf-8") if body else None
    req = urllib.request.Request(url, data=data, method=method)
    if headers:
        for k, v in headers.items():
            req.add_header(k, v)
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status, resp.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8")
    except urllib.error.URLError as e:
        raise RuntimeError(f"Network error: {e.reason}") from e


# ---------------------------------------------------------------------------
# Linear client
# ---------------------------------------------------------------------------

def _linear_create_issue(api_key, team_id, title, description):
    """Cria issue no Linear via GraphQL. Retorna identifier (ex: 'ENG-123')."""
    query = """mutation IssueCreate($title: String!, $description: String!, $teamId: String!) {
    issueCreate(input: {title: $title, description: $description, teamId: $teamId}) {
      success
      issue { id identifier }
    }
  }"""
    payload = json.dumps({
        "query": query,
        "variables": {"title": title, "description": description, "teamId": team_id},
    })
    status, body = _http_request(
        "https://api.linear.app/graphql",
        method="POST",
        headers={
            "Content-Type": "application/json",
            "Authorization": api_key,
        },
        body=payload,
    )
    if status != 200:
        raise RuntimeError(f"Linear: unexpected status {status}: {body}")
    data = json.loads(body)
    if data.get("errors"):
        raise RuntimeError(f"Linear API error: {data['errors'][0]['message']}")
    result = data["data"]["issueCreate"]
    if not result.get("success"):
        raise RuntimeError("Linear: issueCreate returned success=false")
    return result["issue"]["identifier"]


# ---------------------------------------------------------------------------
# Jira client
# ---------------------------------------------------------------------------

def _jira_create_issue(base_url, email, token, project, title, description):
    """Cria issue no Jira via REST API v3. Retorna issue key (ex: 'ENG-456')."""
    payload = json.dumps({
        "fields": {
            "project": {"key": project},
            "summary": title,
            "description": {
                "type": "doc",
                "version": 1,
                "content": [{
                    "type": "paragraph",
                    "content": [{"type": "text", "text": description}],
                }],
            },
            "issuetype": {"name": "Story"},
        }
    })
    creds = base64.b64encode(f"{email}:{token}".encode()).decode()
    url = base_url.rstrip("/") + "/rest/api/3/issue"
    status, body = _http_request(
        url,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "Accept": "application/json",
            "Authorization": f"Basic {creds}",
        },
        body=payload,
    )
    if status != 201:
        raise RuntimeError(f"Jira: unexpected status {status}: {body}")
    data = json.loads(body)
    if not data.get("key"):
        raise RuntimeError("Jira: response missing issue key")
    return data["key"]


# ---------------------------------------------------------------------------
# REQ file helpers
# ---------------------------------------------------------------------------

def _is_status_open(text):
    for line in text.split("\n"):
        if "| Status:" in line:
            return "Status: Open" in line
    return False


def _extract_field(text, field):
    prefix = "| " + field + ":"
    for line in text.split("\n"):
        trimmed = line.strip()
        if trimmed.startswith(prefix):
            return trimmed[len(prefix):].strip()
    return ""


def _extract_title(text):
    for line in text.split("\n"):
        if line.startswith("# REQ: "):
            return line[len("# REQ: "):]
    return ""


def _extract_motivation(text):
    lines = text.split("\n")
    in_section = False
    parts = []
    for line in lines:
        if line.startswith("## Motivation") or line.startswith("## Motivação"):
            in_section = True
            continue
        if in_section:
            if line.startswith("## "):
                break
            parts.append(line)
    return "\n".join(parts).strip()


def _inject_field(text, field, value):
    """Injeta ou substitui campo na tabela de status do REQ."""
    prefix = "| " + field + ":"
    lines = text.split("\n")
    # se campo já existe, substituir
    for i, line in enumerate(lines):
        if line.strip().startswith(prefix):
            lines[i] = f"| {field}: {value}"
            return "\n".join(lines)
    # inserir após a linha com | Status:
    for i, line in enumerate(lines):
        if "| Status:" in line:
            lines.insert(i + 1, f"| {field}: {value}")
            return "\n".join(lines)
    return text


# ---------------------------------------------------------------------------
# Core sync logic
# ---------------------------------------------------------------------------

def _sync_to_provider(create_fn, issue_field, req_dir="docs/req"):
    """
    Percorre REQs em req_dir/, pula não-Open e já sincronizados,
    chama create_fn(title, desc) e injeta o issue id no arquivo.
    Retorna lista de {req_path, issue_id?, skipped?, error?}.
    """
    results = []
    try:
        files = [
            os.path.join(req_dir, f)
            for f in os.listdir(req_dir)
            if f.endswith(".md")
        ]
    except OSError:
        return []

    for file_path in files:
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                text = f.read()
        except OSError as e:
            results.append({"req_path": file_path, "skipped": False, "error": e})
            continue

        if not _is_status_open(text):
            results.append({"req_path": file_path, "skipped": True})
            continue

        if _extract_field(text, issue_field):
            results.append({"req_path": file_path, "skipped": True})
            continue

        title = _extract_title(text)
        desc = _extract_motivation(text)

        try:
            issue_id = create_fn(title, desc)
            updated = _inject_field(text, issue_field, issue_id)
            with open(file_path, "w", encoding="utf-8") as f:
                f.write(updated)
            results.append({"req_path": file_path, "issue_id": issue_id, "skipped": False})
        except Exception as e:
            results.append({"req_path": file_path, "skipped": False, "error": e})

    return results


def _sync_to_linear():
    api_key = _get_config("linear_api_key", "LINEAR_API_KEY")
    team_id = _get_config("linear_team_id", "LINEAR_TEAM_ID")
    if not api_key:
        raise RuntimeError(
            "Linear API key not found. Set LINEAR_API_KEY env var or linear_api_key in trackfw.yaml"
        )
    if not team_id:
        raise RuntimeError(
            "Linear Team ID not found. Set LINEAR_TEAM_ID env var or linear_team_id in trackfw.yaml"
        )
    return _sync_to_provider(
        lambda title, desc: _linear_create_issue(api_key, team_id, title, desc),
        "linear_issue",
    )


def _sync_to_jira():
    base_url = _get_config("jira_base_url", "JIRA_BASE_URL")
    email = _get_config("jira_email", "JIRA_EMAIL")
    token = _get_config("jira_token", "JIRA_TOKEN")
    project = _get_config("jira_project", "JIRA_PROJECT")
    if not base_url:
        raise RuntimeError(
            "Jira base URL not found. Set JIRA_BASE_URL env var or jira_base_url in trackfw.yaml"
        )
    if not email:
        raise RuntimeError(
            "Jira email not found. Set JIRA_EMAIL env var or jira_email in trackfw.yaml"
        )
    if not token:
        raise RuntimeError(
            "Jira API token not found. Set JIRA_TOKEN env var or jira_token in trackfw.yaml"
        )
    if not project:
        raise RuntimeError(
            "Jira project key not found. Set JIRA_PROJECT env var or jira_project in trackfw.yaml"
        )
    return _sync_to_provider(
        lambda title, desc: _jira_create_issue(base_url, email, token, project, title, desc),
        "jira_issue",
    )


# ---------------------------------------------------------------------------
# Command registration
# ---------------------------------------------------------------------------

def register(subparsers):
    """Adiciona subcomando `sync` ao parser principal."""
    parser = subparsers.add_parser(
        "sync",
        help="Sync Open REQs to a project management tool (Linear, Jira)",
    )
    parser.add_argument(
        "--to",
        required=True,
        choices=["linear", "jira"],
        metavar="TARGET",
        help="Target PM tool: linear or jira",
    )
    parser.set_defaults(func=_cmd_sync)


def _cmd_sync(args):
    target = args.to

    try:
        if target == "linear":
            results = _sync_to_linear()
        elif target == "jira":
            results = _sync_to_jira()
        else:
            print(f'Unknown target "{target}" — use --to=linear or --to=jira', file=sys.stderr)
            sys.exit(1)
    except RuntimeError as e:
        print(str(e), file=sys.stderr)
        sys.exit(1)

    if not results:
        print("No REQs found in docs/req/")
        return

    col = 55
    print(f"{'REQ':<{col}} ISSUE")
    print(f"{'-' * col} {'-' * 10}")
    for r in results:
        if r.get("skipped"):
            print(f"{r['req_path']:<{col}} (skipped)")
        elif r.get("error"):
            print(f"{r['req_path']:<{col}} ERROR: {r['error']}")
        else:
            print(f"{r['req_path']:<{col}} {r['issue_id']}")
