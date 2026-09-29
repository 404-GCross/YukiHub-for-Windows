import type { appconf, vo } from "../../src/bindings/models";

import {
  Disconnect,
  GetAuthStatus,
  GetProfile,
  StartAuth,
} from "../../bindings/yukihub/internal/service/nextmoeservice";

export const NEXTMOE_AUTH_STATUS_EVENT = "nextmoe:auth-status-changed";

export type NextMoeAuthViewState = "unauthorized" | "authorized";

export type NextMoeAuthStatus = {
  state: NextMoeAuthViewState;
  accountLabel: string;
  expiresAt?: string;
};

function readString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function getStatusFromConfig(config: appconf.AppConfig): NextMoeAuthStatus {
  const accessToken = readString(config.nextmoe_access_token);
  const refreshToken = readString(config.nextmoe_refresh_token);

  return {
    state: accessToken || refreshToken ? "authorized" : "unauthorized",
    accountLabel: readString(config.nextmoe_account_label),
    expiresAt: readString(config.nextmoe_token_expires_at) || undefined,
  };
}

function getStatusFromSnapshot(
  snapshot: vo.NextMoeAuthStatus,
): NextMoeAuthStatus {
  return {
    state: snapshot.authorized ? "authorized" : "unauthorized",
    accountLabel: readString(snapshot.account_label),
    expiresAt: readString(snapshot.access_token_expires_at) || undefined,
  };
}

export function mergeNextMoeAuthStatus(
  config: appconf.AppConfig | null,
  snapshot?: vo.NextMoeAuthStatus | null,
): NextMoeAuthStatus {
  if (snapshot) {
    return getStatusFromSnapshot(snapshot);
  }
  if (!config) {
    return { state: "unauthorized", accountLabel: "" };
  }
  return getStatusFromConfig(config);
}

export function fetchNextMoeAuthStatus(): Promise<vo.NextMoeAuthStatus> {
  return GetAuthStatus();
}

export function fetchNextMoeProfile(): Promise<vo.NextMoeProfile> {
  return GetProfile();
}

export function startNextMoeAuthorization(): Promise<vo.NextMoeAuthStatus> {
  return StartAuth();
}

export function disconnectNextMoeAuthorization(): Promise<vo.NextMoeAuthStatus> {
  return Disconnect();
}
