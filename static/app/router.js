const DEFAULT_ROUTE = "/home";

export function normalizeRoute(raw) {
  let path = (raw || "").trim();
  if (path.startsWith("#")) path = path.slice(1);
  if (path === "") return DEFAULT_ROUTE;
  if (!path.startsWith("/")) path = `/${path}`;
  return path;
}

export function getCurrentRoute() {
  return normalizeRoute(window.location.hash);
}

export function navigate(path) {
  const target = normalizeRoute(path);
  if (normalizeRoute(window.location.hash) === target) return;
  window.location.hash = `#${target}`;
}

export function ensureDefaultRoute() {
  const cur = normalizeRoute(window.location.hash);
  if (!window.location.hash || cur === DEFAULT_ROUTE) {
    window.location.hash = `#${DEFAULT_ROUTE}`;
  }
}

export function onRouteChange(cb) {
  const handler = () => cb(getCurrentRoute());
  window.addEventListener("hashchange", handler);
  return () => window.removeEventListener("hashchange", handler);
}

export { DEFAULT_ROUTE };

