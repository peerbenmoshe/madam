// SPIKE ONLY. Records every Content-Security-Policy violation the browser
// reports, so the page can show them (phones have no console to read).
// Imported first in main.tsx, before anything renders.

export type Violation = { directive: string; blocked: string; sample: string };

const violations: Violation[] = [];
const listeners = new Set<() => void>();

document.addEventListener("securitypolicyviolation", (e) => {
  violations.push({
    directive: e.violatedDirective,
    blocked: e.blockedURI,
    sample: e.sample,
  });
  listeners.forEach((l) => l());
});

export function getViolations(): readonly Violation[] {
  return violations;
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

// The negative control: writes a style attribute as markup, which the CSP
// must refuse. One click must add exactly one violation; if it adds none,
// the listener or the CSP isn't working and a zero means nothing.
export function provokeViolation(): void {
  const span = document.createElement("span");
  document.body.appendChild(span);
  span.setAttribute("style", "outline: 0");
  span.remove();
}
