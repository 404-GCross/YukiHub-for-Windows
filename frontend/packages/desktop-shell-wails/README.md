# Desktop Shell Wails

Wails v3 adapter for `@yukihub/desktop-shell-react`.

Depends on `@yukihub/desktop-shell-core` and Wails beta.24, with no React dependency.
Vue and plain JavaScript applications can consume the same adapter.

```tsx
import { DesktopShellProvider } from "@yukihub/desktop-shell-react";
import { createWailsDesktopAdapter } from "@yukihub/desktop-shell-wails";

const adapter = createWailsDesktopAdapter();

function App() {
  return (
    <DesktopShellProvider adapter={adapter}>
      <Application />
    </DesktopShellProvider>
  );
}
```

The adapter maps Wails platform names, window commands, maximized state, and
`--wails-draggable` regions to the shared React contract.
