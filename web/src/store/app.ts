import { defineStore } from "pinia";

export type AppLanguage = "zh" | "en";
export type AppTheme = "light" | "dark";

const LS_KEY = "bastion_admin_settings_v1";

type Persisted = {
  sidebarCollapsed: boolean;
  language: AppLanguage;
  theme: AppTheme;
  manualUpdateProxy?: string;
};

function loadPersisted(): Persisted {
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (!raw) return { sidebarCollapsed: false, language: "zh", theme: "light" };
    const parsed = JSON.parse(raw) as Partial<Persisted>;

    const theme: AppTheme = parsed.theme === "dark" ? "dark" : "light";

    return {
      sidebarCollapsed: Boolean(parsed.sidebarCollapsed),
      language: parsed.language === "en" ? "en" : "zh",
      theme,
      manualUpdateProxy:
        typeof parsed.manualUpdateProxy === "string" ? parsed.manualUpdateProxy : undefined,
    };
  } catch {
    return { sidebarCollapsed: false, language: "zh", theme: "light" };
  }
}

function savePersisted(next: Persisted) {
  localStorage.setItem(LS_KEY, JSON.stringify(next));
}

function applyThemeToDocument(theme: AppTheme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

export const useAppStore = defineStore("app", {
  state: () => loadPersisted(),
  actions: {
    initTheme() {
      applyThemeToDocument(this.theme);
    },
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed;
      savePersisted(this.$state);
    },
    setLanguage(lang: AppLanguage) {
      this.language = lang;
      savePersisted(this.$state);
    },
    setTheme(theme: AppTheme) {
      this.theme = theme;
      applyThemeToDocument(theme);
      savePersisted(this.$state);
    },
    toggleTheme() {
      this.setTheme(this.theme === "dark" ? "light" : "dark");
    },
    setManualUpdateProxy(value: string) {
      this.manualUpdateProxy = value;
      savePersisted(this.$state);
    },
    clearManualUpdateProxy() {
      this.manualUpdateProxy = undefined;
      savePersisted(this.$state);
    },
  },
});
