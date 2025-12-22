export async function apiJSON(path, method = "GET", body, options = {}) {
  const url = path.startsWith("/api") ? path : `/api${path}`;
  const init = {
    method,
    headers: { "Content-Type": "application/json" },
    ...options,
  };
  if (body !== undefined) {
    init.body = JSON.stringify(body);
  }

  const resp = await fetch(url, init);
  const isJSON = (resp.headers.get("content-type") || "").includes(
    "application/json"
  );
  const payload = isJSON ? await resp.json().catch(() => ({})) : null;

  if (!resp.ok) {
    const detail =
      (payload && (payload.detail || payload.message)) || resp.statusText;
    throw new Error(detail);
  }
  return payload;
}

export function apiGET(path, options) {
  return apiJSON(path, "GET", undefined, options);
}

export function apiPOST(path, body, options) {
  return apiJSON(path, "POST", body, options);
}

export function apiPUT(path, body, options) {
  return apiJSON(path, "PUT", body, options);
}

export function apiDELETE(path, options) {
  return apiJSON(path, "DELETE", undefined, options);
}
