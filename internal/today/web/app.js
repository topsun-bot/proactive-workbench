function weatherLineFrom(data) {
  // Client uses WeatherAvailable only. Never treat a missing/zero temp as live weather.
  if (data.WeatherAvailable === true) {
    return data.WeatherLine || "";
  }
  return data.WeatherUserMessage || data.WeatherLine || "天气暂时查不到";
}

function calendarMessageFrom(data) {
  // Explicit CalendarStatus — never infer from an empty Tasks/events list.
  switch (data.CalendarStatus) {
    case "available":
      return data.CalendarUserMessage || "";
    case "permission_denied":
      return data.CalendarUserMessage || "日历权限被拒绝";
    case "unavailable":
      return data.CalendarUserMessage || "日历暂时读不到";
    case "unconfigured":
      return data.CalendarUserMessage || "日历未配置";
    default:
      return data.CalendarUserMessage || "日历未配置";
  }
}

function applyCalendarStatus(status, message, events) {
  const data = {
    CalendarStatus: status,
    CalendarUserMessage: message || "",
  };
  const note = document.getElementById("calendar-status");
  if (note) {
    const text = calendarMessageFrom(data);
    note.textContent = text;
    note.hidden = !text;
    note.dataset.status = status || "";
  }
  if (status === "available" && Array.isArray(events)) {
    const tasks = document.getElementById("tasks");
    if (!tasks) {
      return;
    }
    events.forEach((e) => {
      const art = document.createElement("article");
      art.className = "task";
      const time = document.createElement("time");
      time.textContent = e.start || e.Start || "";
      art.appendChild(time);
      const h = document.createElement("h3");
      h.textContent = e.title || e.Title || "";
      art.appendChild(h);
      const k = document.createElement("div");
      k.className = "kind";
      k.textContent = "eventkit";
      art.appendChild(k);
      tasks.appendChild(art);
    });
  }
}

window.applyCalendarStatus = applyCalendarStatus;

async function load() {
  const params = new URLSearchParams(window.location.search);
  const qs = new URLSearchParams();
  if (params.get("now")) qs.set("now", params.get("now"));
  if (params.get("weather")) qs.set("weather", params.get("weather"));
  if (params.get("debug-fixture")) qs.set("debug-fixture", params.get("debug-fixture"));
  const res = await fetch("/api/today?" + qs.toString());
  const data = await res.json();
  document.getElementById("greeting").textContent = data.Greeting || "Today";
  document.getElementById("date").textContent = data.DateLabel || "";
  document.getElementById("briefing").textContent = data.Briefing || "";

  const wxEl = document.getElementById("weather-line");
  if (wxEl) {
    wxEl.textContent = weatherLineFrom(data);
    wxEl.dataset.available = data.WeatherAvailable === true ? "true" : "false";
  }

  applyCalendarStatus(data.CalendarStatus, data.CalendarUserMessage, null);

  const pill = document.getElementById("mode-pill");
  if (pill) {
    const mock = data.WeatherMock || (data.MemorySource && data.MemorySource.indexOf("MOCK") !== -1);
    if (mock) {
      pill.hidden = false;
      pill.textContent = "MOCK data";
    } else {
      pill.hidden = true;
    }
  }

  const tasks = data.Tasks || [];
  document.getElementById("tasks").innerHTML = tasks.length
    ? tasks.map((t) => {
        const hh = String(t.Hour).padStart(2, "0");
        const mm = String(t.Minute).padStart(2, "0");
        return `<article class="task"><time>${hh}:${mm}</time><h3>${esc(t.Title)}</h3><div class="kind">${esc(t.Kind)}</div></article>`;
      }).join("")
    : `<p class="date" id="tasks-empty">not configured</p>`;

  document.getElementById("signals").innerHTML = (data.Signals || []).map((s) => {
    const mock = s.Mock ? `<span class="mock">MOCK</span>` : "";
    return `<div class="signal"><span><b>${esc(s.Label)}</b><br>${esc(s.Value)}</span>${mock}</div>`;
  }).join("");

  const routines = data.Routines || [];
  document.getElementById("routines").innerHTML = routines.length
    ? routines.map((r) => {
        const hh = String(r.Hour).padStart(2, "0");
        const mm = String(r.Minute).padStart(2, "0");
        return `<li><span>${esc(r.Title)}</span><span>${hh}:${mm} · ${esc(r.Window)}</span></li>`;
      }).join("")
    : `<li class="date">not configured</li>`;

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
