package cli

import (
	"yukihub/internal/cli/protocolcmd"

	"github.com/spf13/cobra"
)

func newProtocolCmd(_ *CoreApp) *cobra.Command {
	return protocolcmd.NewCommand()
}
