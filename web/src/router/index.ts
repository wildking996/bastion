import { createRouter, createWebHashHistory } from "vue-router";

import AppFrame from "@/layout/AppFrame.vue";
import BastionsPage from "@/pages/BastionsPage.vue";
import ErrorLogsPage from "@/pages/ErrorLogsPage.vue";
import HomePage from "@/pages/HomePage.vue";
import HttpLogsPage from "@/pages/HttpLogsPage.vue";
import MappingsPage from "@/pages/MappingsPage.vue";

export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: "/",
      component: AppFrame,
      children: [
        { path: "", redirect: "/home" },
        { path: "home", component: HomePage },
        { path: "bastions", component: BastionsPage },
        { path: "mappings", component: MappingsPage },
        { path: "logs/http", component: HttpLogsPage },
        { path: "logs/errors", component: ErrorLogsPage },
      ],
    },
  ],
});
