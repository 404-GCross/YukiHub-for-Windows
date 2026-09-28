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

export function SetGameSteamLaunchOptions(
  gameID: string,
  launchOptions: string,
): Promise<service.SteamLaunchStatus> {
  if (integrationService.SetGameSteamLaunchOptions) {
    return integrationService.SetGameSteamLaunchOptions(gameID, launchOptions);
  }
  return missingBinding<service.SteamLaunchStatus>("SetGameSteamLaunchOptions");
}
