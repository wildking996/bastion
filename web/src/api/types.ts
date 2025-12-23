export type Bastion = {
  id: number;
  name: string;
  host: string;
  port: number;
  username: string;
  password?: string;
  pkey_path?: string;
  pkey_passphrase?: string;
};

export type BastionCreate = {
  name?: string;
  host: string;
  port?: number;
  username: string;
  password?: string;
  pkey_path?: string;
  pkey_passphrase?: string;
};

export type MappingRead = {
  id: string;
  local_host: string;
  local_port: number;
  remote_host: string;
  remote_port: number;
  chain: string[];
  allow_cidrs: string[];
  deny_cidrs: string[];
  type: "tcp" | "socks5" | "http" | string;
  auto_start: boolean;
  running: boolean;
};

export type MappingCreate = {
  id?: string;
  local_host?: string;
  local_port: number;
  remote_host?: string;
  remote_port?: number;
  chain?: string[];
  allow_cidrs?: string[];
  deny_cidrs?: string[];
  type?: string;
  auto_start?: boolean;
};

export type UpdateCheckResponse = {
  current_version: string;
  latest_version: string;
  update_available: boolean;
  release_url?: string;
  asset_name?: string;
  download_url?: string;
};

export type UpdateProxyResponse = {
  manual_proxy?: string;
  env_http_proxy?: string;
  env_https_proxy?: string;
  env_all_proxy?: string;
  env_no_proxy?: string;
  effective_proxy?: string;
  source: "manual" | "env" | "none" | string;
};

export type UpdateApplyResponse = {
  ok: boolean;
  target_version: string;
  message: string;
  helper_pid?: number;
  helper_log_path?: string;
};

export type HTTPLog = {
  id: number;
  timestamp: string;
  conn_id: string;
  mapping_id: string;
  local_port: number;
  bastion_chain?: string[];
  method: string;
  url: string;
  host: string;
  protocol: string;
  status_code: number;
  req_size: number;
  resp_size: number;
  is_gzipped: boolean;
  duration_ms: number;
};

export type HTTPLogsPageResponse = {
  data: HTTPLog[];
  page: number;
  page_size: number;
  total: number;
};

export type HTTPLogPartResult = {
  data: string;
  truncated: boolean;
  truncated_reason?: string;
};

export type ErrorLog = {
  id: number;
  timestamp: string;
  level: string;
  source: string;
  message: string;
  detail: string;
  stack: string;
  context: string;
};

export type StatsSnapshot = {
  up_bytes: number;
  down_bytes: number;
  connections: number;
};

export type StatsMap = Record<string, StatsSnapshot>;
