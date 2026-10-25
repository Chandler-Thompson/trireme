// Trireme web UI. Every button here is one API call; nothing happens only in the page.
(() => {
  "use strict";
  const state = { lastLine: 0, lastEvent: 0, token: readToken(), declined: false };

  function readToken() {
    try { return localStorage.getItem("trireme.token") || ""; } catch { return ""; }
  }

  function saveToken(t) {
    try { localStorage.setItem("trireme.token", t); } catch { /* private window: keep it in memory only */ }
  }

  // askToken asks the user once for the whole page, not once per failed request: the
  // page refreshes several things at the same moment, and each one fails together.
  // It does not ask when another request already got a new token since this one was
  // sent, nor after the user cancelled (they click the status line to be asked again).
  function askToken(sentWith) {
    if (state.token !== sentWith || state.declined) return;
    const t = prompt("This Trireme server needs its API token:");
    if (t && t.trim()) { state.token = t.trim(); saveToken(state.token); }
    else state.declined = true;
  }

  async function api(method, path, body, retried) {
    const sentWith = state.token;
    const headers = { "Content-Type": "application/json" };
    if (sentWith) headers.Authorization = "Bearer " + sentWith;
    const res = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined });
    if (res.status === 401) {
      askToken(sentWith);
      if (!retried && state.token !== sentWith) return api(method, path, body, true);
      throw new Error("needs the API token");
    }
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || res.statusText);
    return data;
  }

  function el(tag, attrs, ...children) {
    const node = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      if (k === "class") node.className = v;
      else if (k.startsWith("on")) node.addEventListener(k.slice(2), v);
      else node.setAttribute(k, v);
    }
    for (const c of children) node.append(c);
    return node;
  }

  const speakerName = { user: "You", xo: "XO", system: "Trireme" };

  async function refreshChat() {
    const data = await api("GET", "/api/chat?after=" + state.lastLine);
    const box = document.getElementById("lines");
    const atBottom = box.scrollHeight - box.scrollTop - box.clientHeight < 40;
    for (const l of data.lines) {
      box.append(el("div", { class: "line " + l.speaker }, el("span", { class: "who" }, speakerName[l.speaker] || l.speaker), l.text));
      state.lastLine = l.id;
    }
    document.querySelector("#say button").textContent = data.busy ? "XO is thinking…" : "Send";
    if (atBottom || data.lines.length) box.scrollTop = box.scrollHeight;
  }

  async function refreshBlockers() {
    const data = await api("GET", "/api/blockers?open=1");
    const box = document.getElementById("blockers");
    document.getElementById("blocker-count").textContent = data.blockers.length || "";
    if (box.contains(document.activeElement)) return; // why: do not wipe an answer being typed
    box.replaceChildren(...(data.blockers.length ? data.blockers.map(blockerView) : [el("p", { class: "empty" }, "Nothing is waiting on you.")]));
  }

  function blockerView(b) {
    const hint = b.kind === "cap_reached" ? "New cap in dollars" : b.options.length ? "Optional note" : "Your answer";
    const text = el("input", { type: "text", placeholder: hint, "aria-label": "Answer text" });
    const answer = (choice) => async () => {
      try { await api("POST", "/api/blockers/" + b.id + "/answer", { choice, text: text.value }); await tick(); }
      catch (e) { alert(e.message); }
    };
    const buttons = b.options.length ? b.options.map((o) => el("button", { onclick: answer(o) }, o)) : [el("button", { onclick: answer("") }, "Answer")];
    const kids = [el("div", {}, b.summary), el("div", { class: "meta" }, b.project_id + " · from " + b.seat)];
    if (b.detail) kids.push(el("details", {}, el("summary", {}, "Detail"), b.detail));
    kids.push(el("div", { class: "actions" }, text, ...buttons));
    return el("div", { class: "blocker" }, ...kids);
  }

  async function refreshFleets() {
    const data = await api("GET", "/api/fleets");
    const box = document.getElementById("fleets");
    const rows = data.fleets.map((f) => fleetView(f, data.seats));
    box.replaceChildren(...(rows.length ? rows : [el("p", { class: "empty" }, "No projects are configured.")]));
  }

  const actionsFor = { not_started: ["start"], running: ["pause", "stop"], paused: ["resume", "stop"], stopped: ["start", "archive"] };

  function fleetView(f, seats) {
    const act = (a) => async () => {
      try { await api("POST", "/api/fleets/" + encodeURIComponent(f.project_id) + "/" + a); await tick(); }
      catch (e) { alert(e.message); }
    };
    const seatText = Object.entries(seats).filter(([k]) => k.startsWith(f.project_id + "/"))
      .map(([k, v]) => k.split("/")[1] + " " + v.toLowerCase()).join(" · ");
    return el("div", { class: "fleet" },
      el("strong", {}, f.project_id),
      el("span", { class: "state " + f.state }, f.state.replace("_", " ") + (f.reason ? " (" + f.reason.replace("_", " ") + ")" : "")),
      el("span", { class: "muted" }, "$" + f.spent_usd.toFixed(2) + (f.cap_usd ? " of $" + f.cap_usd.toFixed(2) : "") + " spent"),
      ...(actionsFor[f.state] || []).map((a) => el("button", { class: "secondary", onclick: act(a) }, a)),
      el("div", { class: "seats" }, seatText));
  }

  async function refreshJournal() {
    const data = await api("GET", "/api/journal?after=" + state.lastEvent + "&limit=200");
    const list = document.getElementById("journal");
    for (const e of data.events) {
      const time = new Date(e.ts).toLocaleTimeString();
      list.prepend(el("li", {}, time + " " + e.project_id + " " + e.seat + " " + e.type));
      state.lastEvent = e.id;
    }
    while (list.children.length > 300) list.lastChild.remove();
  }

  async function tick() {
    const conn = document.getElementById("conn");
    try {
      await Promise.all([refreshChat(), refreshBlockers(), refreshFleets(), refreshJournal()]);
      conn.textContent = "connected";
    } catch (e) {
      conn.textContent = state.declined ? "needs the API token (click to enter it)" : "offline: " + e.message;
    }
  }

  document.getElementById("conn").addEventListener("click", () => {
    if (!state.declined) return;
    state.declined = false;
    askToken(state.token);
    tick();
  });

  document.getElementById("say").addEventListener("submit", async (ev) => {
    ev.preventDefault();
    const box = document.getElementById("text");
    const text = box.value.trim();
    if (!text) return;
    try { await api("POST", "/api/chat", { text }); box.value = ""; await refreshChat(); }
    catch (e) { alert(e.message); }
  });
  document.getElementById("text").addEventListener("keydown", (ev) => {
    if (ev.key === "Enter" && (ev.ctrlKey || ev.metaKey)) document.getElementById("say").requestSubmit();
  });

  tick();
  setInterval(tick, 2000);
})();
