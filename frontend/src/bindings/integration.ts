import type { service } from "./models";

import * as GeneratedIntegrationService from "../../bindings/yukihub/internal/service/integrationservice";

type IntegrationServiceCompat = typeof GeneratedIntegrationService & {
  SetGameSteamLaunchOptions?: (
    gameID: string,
    launchOptions: string,
  ) => Promise<service.SteamLaunchStatus>;
};

const integrationService
  = GeneratedIntegrationService as IntegrationServiceCompat;

function missingBinding<T>(method: string): Promise<T> {
  return Promise.reject(
    new Error(`${method} binding is not generated yet`),
  );
}

export type SteamCompatibilityTool = service.SteamCompatibilityTool;
export type SteamCompatibilityInfo = service.SteamCompatibilityInfo;
export type LocalProtonTool = service.LocalProtonTool;
export type GameCompatibilityToolsInfo = service.GameCompatibilityToolsInfo;

function callOrMissing<T>(
  method: string,
  call: () => Promise<T> | undefined,
): Promise<T> {
  const result = call();
  if (result) {
    return result;
  }
  return missingBinding<T>(method);
}

export function GetGameCompatibilityTools(
  gameID: string,
): Promise<GameCompatibilityToolsInfo> {
  return callOrMissing("GetGameCompatibilityTools", () =>
    integrationService.GetGameCompatibilityTools?.(gameID));
}

export function GetLocalProtonTools(): Promise<LocalProtonTool[]> {
  return callOrMissing("GetLocalProtonTools", () =>
    integrationService.GetLocalProtonTools?.());
}

export function GetGameSteamCompatibility(
  gameID: string,
): Promise<SteamCompatibilityInfo> {
  return callOrMissing("GetGameSteamCompatibility", () =>
    integrationService.GetGameSteamCompatibility?.(gameID));
}

export function SetGameSteamCompatibilityTool(
  gameID: string,
  toolName: string,
): Promise<SteamCompatibilityInfo> {
  return callOrMissing("SetGameSteamCompatibilityTool", () =>
    integrationService.SetGameSteamCompatibilityTool?.(gameID, toolName));
}

export function OpenGameCompatibilityTool(
  gameID: string,
  action: string,
): Promise<string> {
  return callOrMissing("OpenGameCompatibilityTool", () =>
    integrationService.OpenGameCompatibilityTool?.(gameID, action));
}

export function OpenGameSteamProtonPrefix(gameID: string): Promise<string> {
  return callOrMissing("OpenGameSteamProtonPrefix", () =>
    integrationService.OpenGameSteamProtonPrefix?.(gameID));
}

export function RestartSteamClient(): Promise<void> {
  return callOrMissing("RestartSteamClient", () =>
    integrationService.RestartSteamClient?.());
}

export function SetGameSteamLaunchOptions(
  gameID: string,
  launchOptions: string,
): Promise<service.SteamLaunchStatus> {
  if (integrationService.SetGameSteamLaunchOptions) {
    return integrationService.SetGameSteamLaunchOptions(gameID, launchOptions);
  }
  return missingBinding<service.SteamLaunchStatus>("SetGameSteamLaunchOptions");
}
