import { useSyncExternalStore } from "react";

// The smallest router that does the job: the address bar's path is the
// state. The server answers every path that isn't a file with this app
// (internal/web), so a reload or a link from an email lands on the right
// screen. No dependency.

// Paths compare without a trailing slash: "/signup/" is "/signup".
function currentPath(): string {
  return window.location.pathname.replace(/\/+$/, "") || "/";
}

function subscribe(onChange: () => void): () => void {
  // popstate fires for the browser's Back and Forward buttons.
  window.addEventListener("popstate", onChange);
  return () => window.removeEventListener("popstate", onChange);
}

// usePath re-renders the component whenever the path changes.
export function usePath(): string {
  return useSyncExternalStore(subscribe, currentPath);
}

// navigate goes to another screen without reloading the page. With
// replace, the current entry is overwritten, so Back doesn't return to it:
// right after logging in, for example.
export function navigate(path: string, options: { replace?: boolean } = {}) {
  if (path === currentPath()) {
    return;
  }
  if (options.replace) {
    window.history.replaceState(null, "", path);
  } else {
    window.history.pushState(null, "", path);
  }
  // pushState and replaceState don't announce themselves, so tell usePath.
  window.dispatchEvent(new PopStateEvent("popstate"));
}
