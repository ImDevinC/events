function updateDateRange(form, changedInput) {
  const startDate = form.querySelector('input[name="date"]');
  const endDate = form.querySelector('input[name="end_date"]');
  const error = form.querySelector(".date-range-error");
  const submit = form.querySelector('button[type="submit"]');
  if (!startDate || !endDate || !error || !submit) return false;

  const previousStartDate = startDate.dataset.previousDate ?? startDate.value;
  if (changedInput === startDate && endDate.value === previousStartDate) {
    endDate.value = startDate.value;
  }
  startDate.dataset.previousDate = startDate.value;

  const invalid = Boolean(startDate.value && endDate.value && endDate.value < startDate.value);
  error.hidden = !invalid;
  endDate.setAttribute("aria-invalid", String(invalid));
  submit.disabled = invalid;
  return invalid;
}

function initializeDateRanges(root = document) {
  if (root instanceof HTMLFormElement && root.matches(".event-form")) updateDateRange(root);
  root.querySelectorAll("form.event-form").forEach((form) => updateDateRange(form));
}

function initializeThemeToggle() {
  const toggle = document.querySelector(".theme-toggle");
  if (!toggle) return;

  const systemTheme = window.matchMedia("(prefers-color-scheme: dark)");
  const getTheme = () => document.documentElement.dataset.theme || (systemTheme.matches ? "dark" : "light");
  const updateToggle = () => {
    const isDark = getTheme() === "dark";
    const label = `Switch to ${isDark ? "light" : "dark"} mode`;
    toggle.textContent = isDark ? "☀" : "☾";
    toggle.setAttribute("aria-label", label);
    toggle.setAttribute("title", label);
    toggle.setAttribute("aria-pressed", String(isDark));
  };

  toggle.addEventListener("click", () => {
    const theme = getTheme() === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = theme;
    try {
      localStorage.setItem("events-theme", theme);
    } catch {}
    updateToggle();
  });

  systemTheme.addEventListener("change", () => {
    if (!document.documentElement.dataset.theme) updateToggle();
  });
  updateToggle();
}

document.addEventListener("DOMContentLoaded", () => {
  initializeDateRanges();
  initializeThemeToggle();
});
document.addEventListener("htmx:afterSwap", (event) => initializeDateRanges(event.target));

document.addEventListener("change", (event) => {
  const input = event.target;
  if (!(input instanceof HTMLInputElement) || !["date", "end_date"].includes(input.name)) return;
  const form = input.closest("form.event-form");
  if (form) updateDateRange(form, input);
});

document.addEventListener("submit", (event) => {
  const form = event.target;
  if (!(form instanceof HTMLFormElement) || !form.matches(".event-form")) return;
  if (updateDateRange(form)) event.preventDefault();
}, true);
