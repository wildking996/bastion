export const GROUPS = [
  { key: "home", titleKey: "navHome" },
  { key: "manage", titleKey: "navManage" },
  { key: "logs", titleKey: "navLogs" },
  { key: "system", titleKey: "navSystem" },
];

export const VIEWS = [
  {
    path: "/home",
    group: "home",
    titleKey: "navHomeUpdates",
    loader: () => import("../views/home.js"),
  },
  {
    path: "/bastions",
    group: "manage",
    titleKey: "navBastions",
    loader: () => import("../views/bastions.js"),
  },
  {
    path: "/mappings",
    group: "manage",
    titleKey: "navMappings",
    loader: () => import("../views/mappings.js"),
  },
  {
    path: "/logs/http",
    group: "logs",
    titleKey: "navHTTPLogs",
    loader: () => import("../views/logs_http.js"),
  },
  {
    path: "/logs/errors",
    group: "logs",
    titleKey: "navErrorLogs",
    loader: () => import("../views/logs_errors.js"),
  },
  {
    path: "/system/update",
    group: "system",
    titleKey: "navUpdate",
    loader: () => import("../views/system_update.js"),
  },
];

export function getViewByPath(path) {
  return VIEWS.find((v) => v.path === path);
}

export function getViewsByGroup(groupKey) {
  return VIEWS.filter((v) => v.group === groupKey);
}
