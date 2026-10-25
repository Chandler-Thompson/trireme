// Drives the real page against a real server that requires its API token, the way it
// runs off the server's own machine. Every check counts token prompts a user would see.
import { JSDOM } from "jsdom";
const [base, token] = process.argv.slice(2);
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const html = await (await fetch(base + "/")).text();

function open(answer) {
  const asked = { n: 0 };
  const dom = new JSDOM(html, {
    url: base + "/", runScripts: "dangerously", resources: "usable",
    beforeParse(w) {
      w.fetch = (p, o) => fetch(new URL(p, base), o);
      w.alert = () => {};
      w.prompt = () => { asked.n++; return answer(); };
    },
  });
  return { d: dom.window.document, asked, close: () => dom.window.close() };
}
const until = async (what, fn, ms = 15000) => {
  const end = Date.now() + ms;
  while (Date.now() < end) { if (fn()) return; await wait(100); }
  throw new Error("timed out waiting for " + what);
};
const results = [];
const pass = (name) => { results.push(name); console.log("PASS", name); };

{
  const p = open(() => token);
  await until("connected", () => p.d.getElementById("conn").textContent === "connected");
  await wait(4500);
  if (p.asked.n !== 1) throw new Error(`asked for the token ${p.asked.n} times, want 1`);
  p.close();
  pass("a right token is asked for once, then the page connects");
}
{
  const p = open(() => null);
  await until("asked", () => p.asked.n > 0);
  await wait(4500);
  if (p.asked.n !== 1) throw new Error(`asked ${p.asked.n} times after a cancel, want 1`);
  const conn = p.d.getElementById("conn");
  if (!conn.textContent.includes("token")) throw new Error("status does not say a token is needed: " + conn.textContent);
  conn.click();
  if (p.asked.n !== 2) throw new Error("clicking the status did not ask again");
  p.close();
  pass("a cancelled prompt stays quiet until the user clicks the status");
}
{
  let n = 0;
  const p = open(() => (++n === 1 ? "wrong" : token));
  await until("connected", () => p.d.getElementById("conn").textContent === "connected");
  await wait(4500);
  if (p.asked.n !== 2) throw new Error(`asked ${p.asked.n} times for wrong-then-right, want 2`);
  p.close();
  pass("a wrong token is asked for again once per refresh, not once per request");
}
console.log(results.length + " token checks passed");
