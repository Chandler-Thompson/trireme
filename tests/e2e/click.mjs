// Drives the real page against the real server: every check is a user action.
import { JSDOM } from "jsdom";
const base = process.argv[2];
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const html = await (await fetch(base + "/")).text();
const dom = new JSDOM(html, {
  url: base + "/", runScripts: "dangerously", resources: "usable",
  beforeParse(w) {
    w.fetch = (p, o) => fetch(new URL(p, base), o);
    w.alert = (m) => { console.log("ALERT:", m); };
    w.prompt = () => null;
  },
});
const w = dom.window, d = w.document;
const until = async (what, fn, ms = 15000) => {
  const end = Date.now() + ms;
  while (Date.now() < end) { const v = fn(); if (v) return v; await wait(100); }
  throw new Error("timed out waiting for " + what);
};
const button = (text) => [...d.querySelectorAll("button")].find((b) => b.textContent === text);
const results = [];
const pass = (name) => { results.push(name); console.log("PASS", name); };

await until("connected", () => d.getElementById("conn").textContent === "connected");
pass("page loads and connects");

(await until("start button", () => button("start"))).click();
await until("running", () => d.querySelector(".fleet .state")?.textContent.startsWith("running"));
pass("Start button starts the fleet");

d.getElementById("text").value = "hello XO";
d.getElementById("say").dispatchEvent(new w.Event("submit", { cancelable: true }));
await until("xo reply", () => d.querySelector(".line.xo")?.textContent.includes("XO here"));
pass("chat sends and shows the XO reply");

const kick = await fetch(base + "/api/inbox", { method: "POST", body: JSON.stringify({ project_id: "smoke", to_seat: "technical-lead", from_seat: "user", kind: "kick", body: "go" }) });
if (!kick.ok) throw new Error("kick failed");
await until("seat blocker", () => [...d.querySelectorAll(".blocker")].some((b) => b.textContent.includes("approve the plan")));
pass("a seat's blocker appears in the queue");

const seatBlocker = [...d.querySelectorAll(".blocker")].find((b) => b.textContent.includes("approve the plan"));
[...seatBlocker.querySelectorAll("button")].find((b) => b.textContent === "Approve").click();
await until("blocker cleared", () => ![...d.querySelectorAll(".blocker")].some((b) => b.textContent.includes("approve the plan")));
pass("Approve button answers the blocker");

for (let i = 0; i < 3; i++) {
  await fetch(base + "/api/inbox", { method: "POST", body: JSON.stringify({ project_id: "smoke", to_seat: "technical-lead", from_seat: "user", kind: "kick", body: "again" }) });
  await wait(4500);
}
const capBlocker = await until("cap blocker", () => [...d.querySelectorAll(".blocker")].find((b) => b.textContent.includes("project cap")), 30000);
await until("paused", () => d.querySelector(".fleet .state")?.textContent.includes("paused"));
pass("hitting the cap pauses the fleet and opens a cap blocker");

const input = capBlocker.querySelector("input");
if (input.placeholder !== "New cap in dollars") throw new Error("cap hint missing: " + input.placeholder);
input.value = "5";
[...capBlocker.querySelectorAll("button")].find((b) => b.textContent === "Raise cap and resume").click();
await until("resumed", () => d.querySelector(".fleet .state")?.textContent.startsWith("running"));
await until("cap shown", () => d.querySelector(".fleet .muted")?.textContent.includes("of $5.00"));
pass("raising the cap from the blocker resumes the fleet");

(await until("pause button", () => button("pause"))).click();
await until("paused by user", () => d.querySelector(".fleet .state")?.textContent.includes("(user)"));
button("resume").click();
await until("running again", () => d.querySelector(".fleet .state")?.textContent.startsWith("running"));
pass("Pause and Resume buttons work");

const journal = d.querySelectorAll("#journal li").length;
if (journal < 10) throw new Error("journal feed too short: " + journal);
pass("journal feed shows events");
console.log(results.length + " checks passed");
w.close();
process.exit(0);
