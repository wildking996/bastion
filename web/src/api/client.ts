import axios from "axios";
import type { AxiosError } from "axios";
import { ElMessage } from "element-plus";

import { i18n } from "@/plugins/i18n";

type ApiV2Envelope = {
  code: string;
  message: string;
  data: any;
};

function isApiV2Envelope(value: any): value is ApiV2Envelope {
  return (
    value &&
    typeof value === "object" &&
    typeof value.code === "string" &&
    typeof value.message === "string" &&
    "data" in value
  );
}

function t(key: string, params?: Record<string, any>): string {
  return String(i18n.global.t(key as any, params as any));
}

function hasKey(key: string): boolean {
  return i18n.global.te(key);
}

function firstOwner(data: any): { pid?: number; exe?: string; name?: string } | null {
  const owners = data?.owners;
  if (!Array.isArray(owners) || owners.length === 0) return null;
  const o = owners[0];
  if (!o || typeof o !== "object") return null;
  return {
    pid: typeof o.pid === "number" ? o.pid : undefined,
    exe: typeof o.exe === "string" ? o.exe : undefined,
    name: typeof o.name === "string" ? o.name : undefined,
  };
}

function resourceBusyMessage(data: any): string {
  const addr =
    (typeof data?.attempt?.addr === "string" && data.attempt.addr) ||
    (typeof data?.addr === "string" && data.addr) ||
    "";

  const owner = firstOwner(data);
  if (owner?.pid) {
    const exe = (owner.exe || owner.name || "").trim();
    if (addr && exe) return t("apiError.RESOURCE_BUSY_OWNER_ADDR", { addr, pid: owner.pid, exe });
    if (addr) return t("apiError.RESOURCE_BUSY_PID_ADDR", { addr, pid: owner.pid });
    if (exe) return t("apiError.RESOURCE_BUSY_OWNER", { pid: owner.pid, exe });
    return t("apiError.RESOURCE_BUSY_PID", { pid: owner.pid });
  }

  if (addr) return t("apiError.RESOURCE_BUSY_ADDR", { addr });
  return t("apiError.RESOURCE_BUSY");
}

function apiCodeMessage(code: string, data: any): string {
  if (code === "RESOURCE_BUSY") {
    return resourceBusyMessage(data);
  }

  const key = `apiError.${code}`;
  if (hasKey(key)) return t(key);
  return "";
}

function formatV2Error(envelope: ApiV2Envelope): string {
  const data = envelope.data ?? {};
  const detail = typeof data?.detail === "string" ? data.detail.trim() : "";

  const localized = apiCodeMessage(envelope.code, data);
  const base = localized || (envelope.message || "").trim() || t("apiError.UNKNOWN");

  if (envelope.code === "RESOURCE_BUSY") return base;
  if (detail) return `${base}: ${detail}`;
  return base;
}

function formatError(err: unknown): string {
  const axiosErr = err as AxiosError<any>;

  if (axios.isAxiosError(axiosErr)) {
    if (axiosErr.code === "ECONNABORTED") return t("apiError.TIMEOUT");
    if (!axiosErr.response) return t("apiError.NETWORK");

    const payload = axiosErr.response.data;
    if (isApiV2Envelope(payload)) return formatV2Error(payload);

    const detail = payload?.detail;
    if (typeof detail === "string" && detail.trim() !== "") return detail;

    const msg = axiosErr.message;
    if (typeof msg === "string" && msg.trim() !== "") return msg;
    return t("apiError.UNKNOWN");
  }

  const e = err as any;
  if (isApiV2Envelope(e?.api)) return formatV2Error(e.api);
  if (typeof e?.message === "string" && e.message.trim() !== "") return e.message;
  return t("apiError.UNKNOWN");
}

export const api = axios.create({
  baseURL: "/api/v2",
  timeout: 15000,
});

api.interceptors.response.use(
  (res) => {
    if (isApiV2Envelope(res.data)) {
      if (res.data.code === "OK") {
        res.data = res.data.data;
        return res;
      }
      const e = { api: res.data };
      ElMessage.error(formatError(e));
      return Promise.reject(e);
    }
    return res;
  },
  (err) => {
    ElMessage.error(formatError(err));
    return Promise.reject(err);
  }
);
