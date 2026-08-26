"""Board automation (3b) — wood-fired-tasks REST client + doctrine mechanics.

The board is body, not law. This client:
- posts tasks with the decomposition contract fields, enforcing EXCLUSIVE
  FILE-SCOPE at posting time (CONTRACTS/DECOMPOSITION.md — no two open tasks
  own a file)
- performs atomic claims (WFT native optimistic locking)
- records every COMMS message as a board comment (Q6: task messages live as
  board comments)
- emits the task.* / msg.* / verdict.* ledger events (3b is their emitter)

Doctrine task ids ("T-001") are first-class; the WFT numeric id is an internal
mapping the board client owns.
"""

import json
import urllib.request
import urllib.error


MSG_SEATS = {"TASKING": "producer", "VERDICT_SEAT_PREFIX": "verifier"}


class FileScopeConflict(Exception):
    """Posting refused: a file in the new task's scope is owned by an open task."""


class Board:
    def __init__(self, base_url: str, token: str, emitter, project_name: str):
        self.base = base_url.rstrip("/") + "/api/v1"
        self.token = token
        self.emitter = emitter
        self.project_id = self._ensure_project(project_name)
        self._tasks = {}        # doctrine task id -> wft id
        self._file_scopes = {}  # doctrine task id -> set(files), open tasks only

    # ---- HTTP plumbing -------------------------------------------------
    def _req(self, method: str, path: str, body: dict | None = None):
        headers = {"Authorization": f"Bearer {self.token}"}
        if body is not None:
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(
            self.base + path,
            data=json.dumps(body).encode() if body is not None else None,
            method=method,
            headers=headers,
        )
        try:
            with urllib.request.urlopen(req) as resp:
                raw = resp.read()
                return json.loads(raw) if raw else None
        except urllib.error.HTTPError as e:
            detail = e.read().decode(errors="replace")
            raise RuntimeError(f"{method} {path} -> {e.code}: {detail}") from e

    def _ensure_project(self, name: str) -> int:
        projects = self._req("GET", "/projects")
        rows = projects.get("data", projects) if isinstance(projects, dict) else projects
        for p in rows:
            if p["name"] == name:
                return p["id"]
        created = self._req("POST", "/projects", {"name": name, "description": "Trireme sortie board"})
        return created["id"]

    # ---- Doctrine mechanics --------------------------------------------
    def post_task(self, task_id: str, title: str, description: str, files: list,
                  deps: list, budget_tokens: int, budget_wall_s: int, phase: str,
                  acceptance_criteria: str = "", remediation_of=None, synthetic: int = 0):
        # Exclusive file-scope: enforced here, at posting time.
        for other_id, scope in self._file_scopes.items():
            overlap = scope & set(files)
            if overlap:
                raise FileScopeConflict(
                    f"{task_id} claims {sorted(overlap)} already owned by open task {other_id}"
                )
        body = {
            "title": f"{task_id}: {title}",
            "description": description + "\n\nfile-scope: " + json.dumps(files)
                           + "\ndeps: " + json.dumps(deps),
            "project_id": self.project_id,
            "created_by": "producer",
        }
        if acceptance_criteria:
            body["acceptance_criteria"] = acceptance_criteria
        created = self._req("POST", "/tasks", body)
        self._tasks[task_id] = created["id"]
        self._file_scopes[task_id] = set(files)
        self.emitter.emit("producer", task_id, "task.created", {
            "phase": phase, "files": list(files), "deps": list(deps),
            "budget_tokens": budget_tokens, "budget_wall_s": budget_wall_s,
            "remediation_of": remediation_of,
        }, synthetic)
        self.msg("producer", task_id, "TASKING", "-",
                 f"{title}\n{description}", synthetic)
        return created["id"]

    def claim(self, task_id: str, worker_seat: str, synthetic: int = 0):
        wft_id = self._tasks[task_id]
        self._req("POST", f"/tasks/{wft_id}/claim", {"assignee": worker_seat})
        self.emitter.emit(worker_seat, task_id, "task.claimed", {}, synthetic)
        self.msg(worker_seat, task_id, "ACK", "-", "claimed", synthetic)

    def msg(self, seat: str, task_id: str, msg_type: str, prec: str, body: str,
            synthetic: int = 0):
        """COMMS.md v0 message: fixed header, lives as a board comment;
        3b emits msg.sent for every one."""
        wft_id = self._tasks.get(task_id)
        content = f"{msg_type} | {seat} | RE: {task_id} | {prec} | {body}"
        if wft_id is not None:
            self._req("POST", f"/tasks/{wft_id}/comments",
                      {"author": seat, "content": content})
        self.emitter.emit(seat, task_id, "msg.sent",
                          {"msg_type": msg_type, "prec": prec}, synthetic)

    def clarify(self, worker_seat: str, task_id: str, question: str, synthetic: int = 0):
        self.emitter.emit(worker_seat, task_id, "task.clarify",
                          {"question": question}, synthetic)
        self.msg(worker_seat, task_id, "CLARIFY", "-", question, synthetic)

    def clarify_resolved(self, task_id: str, answer: str, synthetic: int = 0):
        self.emitter.emit("producer", task_id, "task.clarify_resolved", {}, synthetic)
        self.msg("producer", task_id, "TASKING", "-", answer, synthetic)

    def completed(self, worker_seat: str, task_id: str, attempt: int, evidence: str,
                  synthetic: int = 0):
        self.emitter.emit(worker_seat, task_id, "task.completed",
                          {"attempt": attempt}, synthetic)
        self.msg(worker_seat, task_id, "SITREP", "ROUTINE",
                 f"DONE attempt {attempt}. Evidence: {evidence}", synthetic)

    def verdict_pass(self, verifier_seat: str, task_id: str, attempt: int,
                     spec_concern: int = 0, synthetic: int = 0):
        self.emitter.emit(verifier_seat, task_id, "verdict.pass",
                          {"attempt": attempt, "spec_concern": spec_concern}, synthetic)
        self.msg(verifier_seat, task_id, "VERDICT", "-", f"PASS attempt {attempt}", synthetic)

    def verdict_fail(self, verifier_seat: str, task_id: str, attempt: int,
                     evidence: str, synthetic: int = 0):
        self.emitter.emit(verifier_seat, task_id, "verdict.fail",
                          {"attempt": attempt, "evidence": evidence}, synthetic)
        self.msg(verifier_seat, task_id, "VERDICT", "-",
                 f"FAIL attempt {attempt}: {evidence}", synthetic)

    def rejected(self, task_id: str, attempt: int, charged: str, synthetic: int = 0):
        self.emitter.emit("board", task_id, "task.rejected",
                          {"attempt": attempt, "charged": charged}, synthetic)

    def reissued(self, task_id: str, synthetic: int = 0):
        wft_id = self._tasks[task_id]
        self._req("PUT", f"/tasks/{wft_id}", {"status": "open", "assignee": None})
        self._file_scopes.pop(task_id, None)
        self.emitter.emit("producer", task_id, "task.reissued", {}, synthetic)

    def merged(self, task_id: str, commit: str, synthetic: int = 0):
        wft_id = self._tasks[task_id]
        self._req("PUT", f"/tasks/{wft_id}",
                  {"status": "done",
                   "verification_evidence": {
                       "verdict": "PASS",
                       "checks": [{"name": "merge", "status": "PASS",
                                   "evidence_url_or_text": f"merged as {commit}"}],
                   }})
        self._file_scopes.pop(task_id, None)  # scope released on merge
        self.emitter.emit("board", task_id, "task.merged", {"commit": commit}, synthetic)

    def close_task_records(self, task_id: str):
        self._file_scopes.pop(task_id, None)
