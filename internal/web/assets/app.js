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

document.addEventListener("DOMContentLoaded", () => initializeDateRanges());
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
