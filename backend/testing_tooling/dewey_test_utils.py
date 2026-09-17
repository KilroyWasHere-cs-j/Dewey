"""Shared helpers for the testing_tooling/*.py scripts (issue #469) — colors,
stdout/stderr log-teeing, and session-token exchange, each previously
duplicated per-script in the bash originals (or, for the session exchange,
missing entirely from load_test.sh/concurrency_test.sh/soak_test.sh, which
still sent the raw X-Dewey-Password header directly. That stopped working
once issue #409 replaced requirePassword with requireSession on every
files/machines route except /session — this module's exchange_session_token
is what actually makes those scripts work again).

Mirrors the same consolidation issue #433 did for cli/dewey-mcp's Go HTTP
client code, applied to this directory's Python scripts instead of
duplicating request-building/auth logic four times over.
"""

from __future__ import annotations

import random
import sys
from datetime import datetime, timedelta

import requests

NC = "\033[0m"
BOLD = "\033[1m"
DIM = "\033[2m"
GREEN = "\033[0;32m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
CYAN = "\033[0;36m"
MAGENTA = "\033[0;35m"
WHITE = "\033[1;37m"


class Tee:
    """Mirrors writes to both the real stream and a log file — Python
    equivalent of the bash scripts' `exec > >(tee -a "$LOG_FILE") 2> >(tee
    -a "$LOG_FILE" >&2)`.
    """

    def __init__(self, stream, log_file):
        self.stream = stream
        self.log_file = log_file

    def write(self, data):
        self.stream.write(data)
        self.log_file.write(data)

    def flush(self):
        self.stream.flush()
        self.log_file.flush()


def exchange_session_token(base: str, password: str, group: str) -> str:
    """Exchanges password for a session token against POST
    /core/<group>/session (issue #409) — group is "files" or "machines",
    matching requireSession's own group parameter (backend/session.go).
    Exits the process with a clear message on failure, same as every
    caller already did for the pre-session-token password check they used
    to do inline.
    """
    try:
        resp = requests.post(
            f"{base}/core/{group}/session",
            headers={"X-Dewey-Password": password},
            timeout=10,
        )
        token = resp.json().get("token", "")
    except (requests.exceptions.RequestException, ValueError):
        token = ""

    if not token:
        print(
            f"Failed to exchange the {group} password for a session token against "
            f"{base} — check the password env var and that the backend is reachable.",
            file=sys.stderr,
        )
        sys.exit(1)
    return token


def rand_date() -> str:
    """A plausible date_of_injury value (required server-side, issue
    #365) — shared by test_suite.py and soak_test.py, which both need
    randomized-but-valid metadata rather than a fixed constant.
    """
    return (datetime(2024, 1, 1) + timedelta(days=random.randint(0, 729))).strftime("%Y-%m-%d")
