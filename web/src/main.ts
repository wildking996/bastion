import { createPinia, setActivePinia } from "pinia";
import { createApp } from "vue";

import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import "element-plus/theme-chalk/dark/css-vars.css";

import * as Icons from "@element-plus/icons-vue";

import App from "@/App.vue";
import { i18n, syncI18nWithStore } from "@/plugins/i18n";
import { router } from "@/router";
import { useAppStore } from "@/store/app";
import "@/styles/index.css";

const app = createApp(App);

const pinia = createPinia();
app.use(pinia);
setActivePinia(pinia);

const appStore = useAppStore();
appStore.initTheme();

function applyDocumentLanguage(lang: string) {
  document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
}

applyDocumentLanguage(appStore.language);
appStore.$subscribe((_mutation, state) => {
  applyDocumentLanguage(state.language);
});

syncI18nWithStore();

app.use(router);
app.use(i18n);
app.use(ElementPlus);

for (const [key, component] of Object.entries(Icons)) {
  app.component(key, component);
}

app.mount("#app");
