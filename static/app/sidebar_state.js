const STORAGE_KEY_GROUPS = "bastion.ui.sidebar.groups.v1";
const STORAGE_KEY_COLLAPSED = "bastion.ui.sidebar.collapsed.v1";

export function loadGroupState(defaults) {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_GROUPS);
    if (!raw) return { ...defaults };
    const parsed = JSON.parse(raw);
    return { ...defaults, ...parsed };
  } catch {
    return { ...defaults };
  }
}

export function saveGroupState(state) {
  try {
    localStorage.setItem(STORAGE_KEY_GROUPS, JSON.stringify(state));
  } catch {
    // ignore
  }
}

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
