package stacktrace

import (
	"github.com/rs/zerolog/log"
	"runtime"
)

//DumpErrorStackAndExit log stacktrace current goroutine of catched error and exit
func DumpErrorStackAndExit(err error) {
	const size = 4096
	stack := make([]byte, size)
	stack = stack[:runtime.Stack(stack, false)]
	_, filePath, line, _ := runtime.Caller(2)

	log.Fatal().Err(err).Str("file", filePath).Int("line", line).Str("stacktrace", string(stack)).Msg("dumpErrorStackAndExit executed")
}
