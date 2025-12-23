import axios from "axios";
import { ElMessage } from "element-plus";

function errorMessage(err: any): string {
  const detail = err?.response?.data?.detail;
  if (typeof detail === "string" && detail.trim() !== "") return detail;
  const msg = err?.message;
  if (typeof msg === "string" && msg.trim() !== "") return msg;
  return "Request failed";
}

export const api = axios.create({
  baseURL: "/api",
  timeout: 15000,
});

api.interceptors.response.use(
  (res) => res,
  (err) => {
    ElMessage.error(errorMessage(err));
    return Promise.reject(err);
  }
);
