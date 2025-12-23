const STORAGE_KEY_COLLAPSED = "bastion.ui.sidebar.collapsed.v1";

export function loadSidebarCollapsed() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_COLLAPSED);
    if (!raw) return false;
    return raw === "true";
  } catch {
    return false;
  }
}

export function saveSidebarCollapsed(collapsed) {
  try {
    localStorage.setItem(STORAGE_KEY_COLLAPSED, collapsed ? "true" : "false");
  } catch {
    // ignore
  }
}
