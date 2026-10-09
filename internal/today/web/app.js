async function load() {
  const params = new URLSearchParams(window.location.search);
  const qs = new URLSearchParams();
  if (params.get("now")) qs.set("now", params.get("now"));
  if (params.get("weather")) qs.set("weather", params.get("weather"));
  const res = await fetch("/api/today?" + qs.toString());
  const data = await res.json();
  document.getElementById("greeting").textContent = data.Greeting || "Today";
  document.getElementById("date").textContent = data.DateLabel || "";
  document.getElementById("briefing").textContent = data.Briefing || "";

  document.getElementById("tasks").innerHTML = (data.Tasks || []).map((t) => {
    const hh = String(t.Hour).padStart(2, "0");
    const mm = String(t.Minute).padStart(2, "0");
    return `<article class="task"><time>${hh}:${mm}</time><h3>${esc(t.Title)}</h3><div class="kind">${esc(t.Kind)}</div></article>`;
  }).join("");

  document.getElementById("signals").innerHTML = (data.Signals || []).map((s) => {
    const mock = s.Mock ? `<span class="mock">MOCK</span>` : "";
    return `<div class="signal"><span><b>${esc(s.Label)}</b><br>${esc(s.Value)}</span>${mock}</div>`;
  }).join("");

  document.getElementById("routines").innerHTML = (data.Routines || []).map((r) => {
    const hh = String(r.Hour).padStart(2, "0");
    const mm = String(r.Minute).padStart(2, "0");
    return `<li><span>${esc(r.Title)}</span><span>${hh}:${mm} · ${esc(r.Window)}</span></li>`;
  }).join("");

  document.getElementById("suggestions").innerHTML = (data.Suggestions || []).map((s) => {
    return `<article class="suggestion"><h3>${esc(s.Title)}</h3><p>${esc(s.Body)}</p><p class="mock">${esc(s.Source)}</p></article>`;
  }).join("") || `<p class="date">No suggestions right now.</p>`;
}

function esc(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[ch]));
}

load().catch((err) => {
  document.getElementById("briefing").textContent = "Failed to load Today: " + err;
});
