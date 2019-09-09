package helpers

import (
	"github.com/rs/zerolog/log"
	"os/exec"
)

//ExecShellCmd exec sh -c cmdString and os.Fatal(1) when exec return exit code > 0
func ExecShellCmd(cmdString, cmdErrMsg string) {
	log.Debug().Msg(cmdString)
	//nolint: gas
	cmd := exec.Command("sh", "-c", cmdString)
	if err := cmd.Run(); err != nil {
		log.Fatal().Err(err).Str("cmdString", cmdString).Msg(cmdErrMsg)
	}
}
