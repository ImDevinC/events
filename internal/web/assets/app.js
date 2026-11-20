document.addEventListener("change", (event) => {
  const checkbox = event.target;
  if (!(checkbox instanceof HTMLInputElement) || checkbox.name !== "multiple_days") return;

  const fields = checkbox.closest(".date-range-fields");
  const endDateField = fields?.querySelector(".end-date-field");
  const endDateInput = endDateField?.querySelector('input[name="end_date"]');
  if (!endDateField || !endDateInput) return;

  endDateField.hidden = !checkbox.checked;
  endDateInput.disabled = !checkbox.checked;
  endDateInput.required = checkbox.checked;
});
